package main

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/benteven/claude-deck/internal/update"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		runLaunch()
		return
	}
	switch os.Args[1] {
	case "preview":
		if len(os.Args) == 2 {
			markBuild()
		}
		runPreview(os.Args[2:])
	case "hook":
		runHook()
	case "reload":
		if err := reload(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "upgrade":
		if err := upgrade(); err != nil {
			client := tmux.Client{Socket: paths.Socket(), Conf: filepath.Join(paths.DeckHome(), "tmux.conf")}
			client.Run("display-message", "-d", "10000", "claude-deck: "+err.Error())
		}
	case "version":
		fmt.Println(version)
	case "ui":
		markBuild()
		if err := ui.Run(version); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func upgrade() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = update.Install(exe, version); err != nil {
		return err
	}
	return exec.Command(exe, "reload").Run()
}

func buildID() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func markBuild() {
	pane, build := os.Getenv("TMUX_PANE"), buildID()
	if pane == "" || build == "" {
		return
	}
	client := tmux.Client{Socket: paths.Socket(), Conf: filepath.Join(paths.DeckHome(), "tmux.conf")}
	if _, err := client.Run("set-option", "-p", "-t", pane, tmux.BuildOption, build); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
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

func setup() (tmux.Client, store.Config, string, error) {
	home := paths.DeckHome()
	cfg, err := store.LoadConfig(home)
	if err != nil {
		return tmux.Client{}, cfg, "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return tmux.Client{}, cfg, "", err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return tmux.Client{}, cfg, "", err
	}
	conf := filepath.Join(home, "tmux.conf")
	if err := tmux.WriteConf(conf, cfg); err != nil {
		return tmux.Client{}, cfg, "", err
	}
	if err := os.WriteFile(filepath.Join(home, "hooks.json"), events.HookSettings(exe), 0o644); err != nil {
		return tmux.Client{}, cfg, "", err
	}
	client := tmux.Client{Socket: paths.Socket(), Conf: conf}
	if _, err := client.Run("list-sessions"); err == nil {
		if _, err := client.Run("source-file", conf); err != nil {
			fmt.Fprintln(os.Stderr, "warning:", err)
		}
	}
	return client, cfg, exe, nil
}

func reload() error {
	client, _, exe, err := setup()
	if err != nil {
		return err
	}
	if !client.HasLayout() {
		return fmt.Errorf("claude-deck is not running")
	}
	return client.Reload(exe, "")
}

func launch() error {
	if os.Getenv("TMUX") != "" {
		cfg, _ := store.LoadConfig(paths.DeckHome())
		return fmt.Errorf("claude-deck uses %s as its key and cannot run inside tmux. Run it in a plain terminal, or set \"prefix\" in %s.", cfg.Prefix, filepath.Join(paths.DeckHome(), "config.json"))
	}
	client, cfg, exe, err := setup()
	if err != nil {
		return err
	}
	if !client.HasLayout() {
		if err := client.EnsureLayout(exe, cfg.ListWidth); err != nil {
			return err
		}
	} else {
		if err := client.SetListWidth(cfg.ListWidth); err != nil {
			return err
		}
		if build := buildID(); build != "" {
			if err := client.Reload(exe, build); err != nil {
				fmt.Fprintln(os.Stderr, "warning:", err)
			}
		}
	}
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"tmux", "-L", client.Socket, "-f", client.Conf, "attach", "-t", tmux.DeckSession}, os.Environ())
}
