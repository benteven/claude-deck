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
	nameRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,23}$`)
	private = map[string]bool{
		".credentials.json":              true,
		globalFile:                       true,
		"remote-settings.json":           true,
		"remote-settings.json.synced-at": true,
		"mcp-needs-auth-cache.json":      true,
		"stats-cache.json":               true,
		"backups":                        true,
		"cache":                          true,
		"telemetry":                      true,
		"statsig":                        true,
	}
)

func ValidName(name string) bool {
	return nameRe.MatchString(name)
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

func Prepare(deckHome, claudeHome, name string) (string, error) {
	if !ValidName(name) {
		return "", fmt.Errorf("account name %q: use a-z, 0-9, - and _", name)
	}
	dir := Dir(deckHome, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(claudeHome)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		link := filepath.Join(dir, e.Name())
		if private[e.Name()] {
			continue
		}
		if _, err := os.Lstat(link); err == nil {
			continue
		}
		if err := os.Symlink(filepath.Join(claudeHome, e.Name()), link); err != nil {
			return "", err
		}
	}
	return dir, seed(filepath.Join(filepath.Dir(claudeHome), globalFile), filepath.Join(dir, globalFile))
}

func seed(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	g := map[string]json.RawMessage{}
	data, err := os.ReadFile(src)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(data, &g); err != nil {
			return err
		}
	}
	for k := range g {
		if k == "oauthAccount" || k == "userID" || strings.Contains(strings.ToLower(k), "cache") {
			delete(g, k)
		}
	}
	out, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dst, out, 0o600)
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
