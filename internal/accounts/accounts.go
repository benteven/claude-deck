package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type Account struct {
	Name, Email string
}

const globalFile = ".claude.json"

var (
	nameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,23}$`)
	required = []string{"projects", "sessions"}
	shared   = append([]string{
		"settings.json", "CLAUDE.md", "keybindings.json", "history.jsonl",
		"agents", "commands", "skills", "plugins", "hooks", "output-styles", "rules",
		"todos", "plans", "tasks", "file-history", "shell-snapshots", "session-env", "paste-cache", "image-cache",
	}, required...)
)

func ValidName(name string) bool {
	return nameRe.MatchString(name) && name != "default"
}

func Dir(deckHome, name string) string {
	return filepath.Join(deckHome, "accounts", name)
}

func Exists(deckHome, name string) bool {
	info, err := os.Stat(Dir(deckHome, name))
	return err == nil && info.IsDir()
}

func List(deckHome, claudeHome string) []Account {
	out := []Account{{Email: Email(filepath.Join(filepath.Dir(claudeHome), globalFile))}}
	entries, _ := os.ReadDir(filepath.Join(deckHome, "accounts"))
	for _, e := range entries {
		if e.IsDir() && ValidName(e.Name()) {
			out = append(out, Account{Name: e.Name(), Email: Email(filepath.Join(Dir(deckHome, e.Name()), globalFile))})
		}
	}
	return out
}

func Email(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	var g struct {
		OAuthAccount struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(data, &g) != nil {
		return ""
	}
	return g.OAuthAccount.EmailAddress
}

func LoggedIn(deckHome, name string) bool {
	return name == "" || Email(filepath.Join(Dir(deckHome, name), globalFile)) != ""
}

func Prepare(deckHome, claudeHome, name string) (string, error) {
	if !ValidName(name) {
		return "", fmt.Errorf("account name %q: use a-z, 0-9, - and _", name)
	}
	dir := Dir(deckHome, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	for _, d := range required {
		if err := os.MkdirAll(filepath.Join(claudeHome, d), 0o755); err != nil {
			return "", err
		}
	}
	for _, e := range shared {
		target, link := filepath.Join(claudeHome, e), filepath.Join(dir, e)
		if _, err := os.Lstat(target); err != nil {
			continue
		}
		if _, err := os.Lstat(link); err == nil {
			continue
		}
		if err := os.Symlink(target, link); err != nil {
			return "", err
		}
	}
	return dir, syncGlobal(filepath.Join(filepath.Dir(claudeHome), globalFile), filepath.Join(dir, globalFile))
}

func Unshared(deckHome, claudeHome, name string) []string {
	var out []string
	for _, e := range shared {
		info, err := os.Lstat(filepath.Join(Dir(deckHome, name), e))
		if err == nil && info.Mode()&os.ModeSymlink == 0 {
			if _, err := os.Lstat(filepath.Join(claudeHome, e)); err == nil {
				out = append(out, e)
			}
		}
	}
	return out
}

func syncGlobal(src, dst string) error {
	from, err := readObject(src)
	if err != nil {
		return err
	}
	to, err := readObject(dst)
	if err != nil {
		return err
	}
	seeded := to == nil
	if seeded {
		to = map[string]json.RawMessage{}
		for k, v := range from {
			if k != "oauthAccount" && k != "userID" && !strings.Contains(strings.ToLower(k), "cache") {
				to[k] = v
			}
		}
	}
	changed := false
	for _, key := range []string{"projects", "mcpServers"} {
		merged, ok, err := mergeEntries(from[key], to[key], key == "projects")
		if err != nil {
			return err
		}
		if ok {
			to[key] = merged
			changed = true
		}
	}
	if !seeded && !changed {
		return nil
	}
	out, err := json.MarshalIndent(to, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".claude.json-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func readObject(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	return m, json.Unmarshal(data, &m)
}

func mergeEntries(from, to json.RawMessage, trust bool) (json.RawMessage, bool, error) {
	if len(from) == 0 {
		return nil, false, nil
	}
	var src, dst map[string]map[string]any
	if err := json.Unmarshal(from, &src); err != nil {
		return nil, false, nil
	}
	if len(to) > 0 {
		if err := json.Unmarshal(to, &dst); err != nil {
			return nil, false, nil
		}
	}
	if dst == nil {
		dst = map[string]map[string]any{}
	}
	changed := false
	for k, v := range src {
		cur, ok := dst[k]
		switch {
		case !ok:
			dst[k] = v
			changed = true
		case trust && v["hasTrustDialogAccepted"] == true && cur["hasTrustDialogAccepted"] != true:
			cur["hasTrustDialogAccepted"] = true
			changed = true
		}
	}
	if !changed {
		return nil, false, nil
	}
	out, err := json.Marshal(dst)
	return out, true, err
}

func ForgetLogins(deckHome string) {
	for _, a := range List(deckHome, "") {
		if a.Name == "" {
			continue
		}
		file := filepath.Join(Dir(deckHome, a.Name), globalFile)
		g, err := readObject(file)
		if err != nil || g == nil {
			continue
		}
		delete(g, "oauthAccount")
		if out, err := json.MarshalIndent(g, "", "  "); err == nil {
			os.WriteFile(file, out, 0o600)
		}
	}
}

func Label(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func Index(list []Account, name string) int {
	return slices.IndexFunc(list, func(a Account) bool { return a.Name == name })
}
