package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	pathNone = iota
	pathOK
	pathMissing
)

type completedMsg struct {
	from, to string
}

func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return home + p[1:]
}

func resolveTyped(v string) (string, int) {
	if v == "" || !strings.ContainsRune("/~.", rune(v[0])) {
		return "", pathNone
	}
	abs, err := filepath.Abs(expandHome(v))
	if err != nil {
		return "", pathMissing
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return abs, pathMissing
	}
	return abs, pathOK
}

func commonPrefix(names []string) string {
	prefix := names[0]
	for _, n := range names[1:] {
		for !strings.HasPrefix(n, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func complete(typed string) tea.Cmd {
	return func() tea.Msg {
		i := strings.LastIndex(typed, "/")
		if i < 0 {
			return nil
		}
		dir, seg := typed[:i+1], typed[i+1:]
		entries, err := os.ReadDir(expandHome(dir))
		if err != nil {
			return nil
		}
		var names []string
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, seg) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(seg, ".")) {
				continue
			}
			if info, err := os.Stat(filepath.Join(expandHome(dir), name)); err == nil && info.IsDir() {
				names = append(names, name)
			}
		}
		if len(names) == 0 {
			return nil
		}
		sort.Strings(names)
		done := commonPrefix(names)
		if len(names) == 1 {
			done += "/"
		}
		if done == seg {
			return nil
		}
		return completedMsg{from: typed, to: dir + done}
	}
}
