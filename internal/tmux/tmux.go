package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/benteven/claude-deck/internal/store"
)

const (
	DeckSession = "=deck"
	BgSession   = "=deck-bg"
	ListPane    = DeckSession + ":main.0"
	Slot        = DeckSession + ":main.1"
	ListWidth   = 36
	NextKey     = "\ue000"
	PrevKey     = "\ue001"
	numberBase  = 0xe001
)

func NumberKey(n int) string {
	return string(rune(numberBase + n))
}

func Number(text string) int {
	r := []rune(text)
	if len(r) != 1 || r[0] <= numberBase || r[0] > numberBase+9 {
		return 0
	}
	return int(r[0] - numberBase)
}

type Client struct {
	Socket, Conf string
}

type Pane struct {
	ID, SessionID, Role, Target string
	PID                         int
	Dead                        bool
}

func (c Client) Run(args ...string) (string, error) {
	full := append([]string{"-L", c.Socket, "-f", c.Conf}, args...)
	out, err := exec.Command("tmux", full...).CombinedOutput()
	text := strings.TrimRight(string(out), "\n")
	if err != nil {
		return text, fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, text)
	}
	return text, nil
}

func (c Client) Panes() ([]Pane, error) {
	format := strings.Join([]string{
		"#{pane_id}", "#{@deck-session}", "#{@deck-role}",
		"#{session_name}:#{window_name}.#{pane_index}", "#{pane_pid}", "#{pane_dead}",
	}, "\t")
	out, err := c.Run("list-panes", "-a", "-F", format)
	if err != nil {
		return nil, err
	}
	var panes []Pane
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			continue
		}
		pid, _ := strconv.Atoi(f[4])
		panes = append(panes, Pane{ID: f[0], SessionID: f[1], Role: f[2], Target: f[3], PID: pid, Dead: f[5] == "1"})
	}
	return panes, nil
}

func (c Client) hasSession(target string) bool {
	_, err := c.Run("has-session", "-t", target)
	return err == nil
}

func (c Client) slotPanes() int {
	out, err := c.Run("list-panes", "-t", DeckSession+":main", "-F", "#{pane_id}")
	if err != nil || out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func (c Client) HasLayout() bool {
	return c.hasSession(DeckSession) && c.hasSession(BgSession) && c.slotPanes() >= 2
}

func (c Client) run(steps ...[]string) error {
	for _, s := range steps {
		if _, err := c.Run(s...); err != nil {
			return err
		}
	}
	return nil
}

func (c Client) EnsureLayout(exe string) error {
	quoted := ShellQuote(exe)
	if !c.hasSession(BgSession) {
		err := c.run(
			[]string{"new-session", "-d", "-s", "deck-bg", "-n", "keep", "-x", "200", "-y", "50", "sh -c 'while :; do sleep 3600; done'"},
			[]string{"set-option", "-p", "-t", BgSession + ":keep", "@deck-role", "keep"},
		)
		if err != nil {
			return err
		}
	}
	if !c.hasSession(DeckSession) {
		err := c.run(
			[]string{"new-session", "-d", "-s", "deck", "-n", "main", "-x", "200", "-y", "50", "sh -c " + ShellQuote("while true; do "+quoted+" ui; sleep 1; done")},
			[]string{"set-option", "-p", "-t", ListPane, "@deck-role", "list"},
		)
		if err != nil {
			return err
		}
	}
	if c.slotPanes() < 2 {
		err := c.run(
			[]string{"split-window", "-h", "-t", ListPane, quoted + " preview"},
			[]string{"set-option", "-p", "-t", Slot, "@deck-role", "preview"},
			[]string{"set-option", "-p", "-t", Slot, "@deck-session", ""},
		)
		if err != nil {
			return err
		}
	}
	return c.run(
		[]string{"resize-pane", "-t", ListPane, "-x", strconv.Itoa(ListWidth)},
		[]string{"select-pane", "-t", ListPane},
	)
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

const listFocused = "#{&&:#{==:#{session_name},deck},#{==:#{pane_index},0}}"

func switchKey(key, code string) string {
	return fmt.Sprintf(`bind -n %s if -F "%s" "if -F '#{@deck-dialog}' 'send-keys -t =deck:main.0 %s' 'send-keys -t =deck:main.1 %s ; select-pane -t =deck:main.1'" "send-keys -t =deck:main.0 -l %s"`, key, listFocused, key, key, code)
}

func numberBinding(n int) string {
	key := fmt.Sprintf("C-%d", n)
	return fmt.Sprintf(`bind -n %s if -F "#{&&:%s,#{@deck-dialog}}" "send-keys -t =deck:main.0 %s" "send-keys -t =deck:main.0 -l %s"`, key, listFocused, key, NumberKey(n))
}

func WriteConf(path string, cfg store.Config) error {
	p := cfg.Prefix
	lines := []string{
		"set -g prefix None",
		"unbind -q -a",
		fmt.Sprintf(`bind -n %s if -F "%s" "send-keys -t =deck:main.1 %s ; select-pane -t =deck:main.1" "select-pane -t =deck:main.0"`, p, listFocused, p),
		switchKey("C-j", NextKey),
		switchKey("C-k", PrevKey),
	}
	for n := 1; n <= 9; n++ {
		lines = append(lines, numberBinding(n))
	}
	lines = append(lines,
		"set -g mouse on",
		"set -g remain-on-exit on",
		"set -g focus-events on",
		"set -g extended-keys always",
		"set -g escape-time 0",
		"set -g default-terminal tmux-256color",
		"set -su terminal-features",
		`set -as terminal-features ",*:RGB"`,
		`set -as terminal-features ",xterm*:extkeys:hyperlinks"`,
		"set -g status off",
		fmt.Sprintf(`set-hook -g window-resized "resize-pane -t =deck:main.0 -x %d"`, ListWidth),
	)
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
