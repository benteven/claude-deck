package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

const (
	pathNone = iota
	pathOK
	pathMissing
)

const maxScan = 2000

type inspectedMsg struct {
	from, path string
	state      int
}

type ghostMsg struct {
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
	if !pathLike(v) {
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
			_, size := utf8.DecodeLastRuneInString(prefix)
			prefix = prefix[:len(prefix)-size]
		}
	}
	return prefix
}

func ghostFor(typed string) string {
	i := strings.LastIndex(typed, "/")
	if i < 0 {
		return ""
	}
	dir, seg := typed[:i+1], typed[i+1:]
	d, err := os.Open(expandHome(dir))
	if err != nil {
		return ""
	}
	defer d.Close()
	entries, _ := d.ReadDir(maxScan)
	var names []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, seg) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(seg, ".")) {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(filepath.Join(expandHome(dir), name))
			isDir = err == nil && info.IsDir()
		}
		if isDir {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	done := commonPrefix(names)
	if len(names) == 1 {
		done += "/"
	}
	if done == seg {
		return ""
	}
	return dir + done
}

func inspect(typed string) tea.Cmd {
	return func() tea.Msg {
		path, state := resolveTyped(typed)
		return inspectedMsg{from: typed, path: path, state: state}
	}
}

func ghost(typed string) tea.Cmd {
	return func() tea.Msg {
		return ghostMsg{from: typed, to: ghostFor(typed)}
	}
}
