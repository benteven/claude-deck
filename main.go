package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/benteven/claude-deck/internal/events"
	"github.com/benteven/claude-deck/internal/paths"
	"github.com/benteven/claude-deck/internal/preview"
	"github.com/benteven/claude-deck/internal/store"
	"github.com/benteven/claude-deck/internal/tmux"
	"github.com/benteven/claude-deck/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		runLaunch()
		return
	}
	switch os.Args[1] {
	case "preview":
		runPreview(os.Args[2:])
	case "hook":
		runHook()
	case "ui":
		if err := ui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func runPreview(args []string) {
	var err error
	if len(args) == 2 && args[0] == "--print" {
		err = preview.Print(args[1])
	} else {
		err = preview.Run(filepath.Join(paths.DeckHome(), "preview-target"))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runHook() {
	if err := events.RunHook(filepath.Join(paths.DeckHome(), "events"), os.Stdin, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func runLaunch() {
	if err := launch(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func launch() error {
	home := paths.DeckHome()
	cfg, err := store.LoadConfig(home)
	if err != nil {
		return err
	}
	if os.Getenv("TMUX") != "" {
		return fmt.Errorf("claude-deck uses %s as its key and cannot run inside tmux. Run it in a plain terminal, or set \"prefix\" in %s.", cfg.Prefix, filepath.Join(home, "config.json"))
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	conf := filepath.Join(home, "tmux.conf")
	if err := tmux.WriteConf(conf, cfg); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(home, "hooks.json"), events.HookSettings(exe), 0o644); err != nil {
		return err
	}
	client := tmux.Client{Socket: paths.Socket(), Conf: conf}
	if _, err := client.Run("list-sessions"); err == nil {
		if _, err := client.Run("source-file", conf); err != nil {
			fmt.Fprintln(os.Stderr, "warning:", err)
		}
	}
	if !client.HasLayout() {
		if err := client.EnsureLayout(exe); err != nil {
			return err
		}
	}
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"tmux", "-L", client.Socket, "-f", conf, "attach", "-t", tmux.DeckSession}, os.Environ())
}
