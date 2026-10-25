package deck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/benteven/claude-deck/internal/paths"
	"github.com/benteven/claude-deck/internal/store"
	"github.com/benteven/claude-deck/internal/tmux"
	"github.com/benteven/claude-deck/internal/transcript"
)

type Ops struct {
	T         tmux.Client
	Cfg       store.Config
	HooksPath string
}

func (o Ops) Start(cwd, sessionID, name string, resume bool) (string, error) {
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return "", fmt.Errorf("folder not found: %s", cwd)
	}
	args := []string{paths.ClaudeCommand()}
	if resume {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}
	if name != "" {
		args = append(args, "--name", name)
	}
	args = append(args, "--settings", o.HooksPath)
	args = append(args, o.Cfg.ClaudeArgs...)
	for i, a := range args {
		args[i] = tmux.ShellQuote(a)
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	command := tmux.ShellQuote(shell) + " -lc " + tmux.ShellQuote("exec "+strings.Join(args, " "))

	newWindow := []string{"new-window", "-d", "-P", "-F", "#{pane_id}", "-t", tmux.BgSession + ":"}
	newWindow = append(newWindow, "-c", strings.ReplaceAll(cwd, "#", "##"), command)
	paneID, err := o.T.Run(newWindow...)
	if err != nil {
		return "", err
	}
	paneID = strings.TrimSpace(paneID)
	if _, err := o.T.Run("set-option", "-p", "-t", paneID, "@deck-session", sessionID); err != nil {
		return paneID, err
	}
	return paneID, nil
}

func (o Ops) Show(paneID string) error {
	current, err := o.T.Run("display-message", "-p", "-t", tmux.Slot, "#{pane_id}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) == paneID {
		return nil
	}
	_, err = o.T.Run("swap-pane", "-d", "-s", paneID, "-t", tmux.Slot)
	return err
}

func (o Ops) Focus(target string) error {
	_, err := o.T.Run("select-pane", "-t", target)
	return err
}

func (o Ops) Stop(paneID string, busy bool) error {
	if busy {
		if _, err := o.T.Run("send-keys", "-t", paneID, "C-c"); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	slot, err := o.T.Run("display-message", "-p", "-t", tmux.Slot, "#{pane_id}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(slot) == paneID {
		return fmt.Errorf("pane %s is still in the slot", paneID)
	}
	out, err := o.T.Run("display-message", "-p", "-t", paneID, "#{pane_dead} #{pane_pid}")
	if err != nil {
		return ignoreGone(err)
	}
	if dead, pid, _ := strings.Cut(strings.TrimSpace(out), " "); dead != "1" {
		if n, err := strconv.Atoi(pid); err == nil && n > 0 {
			syscall.Kill(n, syscall.SIGTERM)
		}
	}
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		out, err := o.T.Run("display-message", "-p", "-t", paneID, "#{pane_dead}")
		if err != nil {
			return ignoreGone(err)
		}
		if strings.TrimSpace(out) == "1" {
			break
		}
	}
	_, err = o.T.Run("kill-pane", "-t", paneID)
	return ignoreGone(err)
}

func ignoreGone(err error) error {
	if err != nil && strings.Contains(err.Error(), "can't find pane") {
		return nil
	}
	return err
}

func (o Ops) StopOutside(pid int, startedAt time.Time, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("no process for this session")
	}
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return nil
	}
	if err := confirmClaude(pid, startedAt); err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return nil
		}
	}
	return fmt.Errorf("session still running after %s, not taken over", timeout)
}

func confirmClaude(pid int, startedAt time.Time) error {
	if startedAt.IsZero() {
		return fmt.Errorf("process %d has no start time, not taken over", pid)
	}
	cmd := exec.Command("ps", "-o", "lstart=,comm=", "-p", strconv.Itoa(pid))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("process %d not found, not taken over", pid)
	}
	f := strings.Fields(string(out))
	if len(f) < 6 {
		return fmt.Errorf("process %d: unexpected ps output %q", pid, out)
	}
	if !strings.Contains(filepath.Base(strings.Join(f[5:], " ")), "claude") {
		return fmt.Errorf("process %d is not claude, not taken over", pid)
	}
	start, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", strings.Join(f[:5], " "), time.Local)
	if err != nil {
		return fmt.Errorf("process %d: %w", pid, err)
	}
	if start.After(startedAt) {
		return fmt.Errorf("process %d started after the session, not taken over", pid)
	}
	return nil
}

func TrashSession(id, transcriptPath string) error {
	if !transcript.ValidID(id) || filepath.Base(transcriptPath) != id+".jsonl" {
		return fmt.Errorf("not a session transcript: %s", transcriptPath)
	}
	paths := []string{transcriptPath}
	dir := filepath.Join(filepath.Dir(transcriptPath), id)
	if info, err := os.Lstat(dir); err == nil && info.IsDir() {
		paths = append(paths, dir)
	}
	return Trash(paths...)
}

func Trash(paths ...string) error {
	args := make([]string, 0, len(paths))
	for _, p := range paths {
		if base := filepath.Base(p); filepath.Clean(p) != p || strings.HasPrefix(base, ".") || base == string(filepath.Separator) {
			return fmt.Errorf("trash: refused %s", p)
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		args = append(args, abs)
	}
	out, err := exec.Command("/usr/bin/trash", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	text, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if _, after, ok := strings.Cut(text, "# "); ok {
		text = after
	}
	if text == "" {
		text = err.Error()
	}
	return fmt.Errorf("trash: %s", text)
}
