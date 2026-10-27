package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareSharesAllButLogin(t *testing.T) {
	root := t.TempDir()
	claudeHome := filepath.Join(root, ".claude")
	for _, d := range []string{"projects", "sessions"} {
		if err := os.MkdirAll(filepath.Join(claudeHome, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"settings.json", ".credentials.json"} {
		if err := os.WriteFile(filepath.Join(claudeHome, f), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	global := `{"oauthAccount":{"emailAddress":"a@b.c"},"modelAccessCache":{},"projects":{"/x":{}}}`
	if err := os.WriteFile(filepath.Join(root, ".claude.json"), []byte(global), 0o600); err != nil {
		t.Fatal(err)
	}

	dir, err := Prepare(filepath.Join(root, "deck"), claudeHome, "work")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"projects", "sessions", "settings.json"} {
		if target, err := os.Readlink(filepath.Join(dir, name)); err != nil || target != filepath.Join(claudeHome, name) {
			t.Errorf("%s: link %q, %v", name, target, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(dir, ".credentials.json")); !os.IsNotExist(err) {
		t.Errorf(".credentials.json is shared: %v", err)
	}
	if got := Email(filepath.Join(dir, ".claude.json")); got != "" {
		t.Errorf("login copied: %q", got)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".claude.json"))
	if want := `{
  "projects": {
    "/x": {}
  }
}`; string(data) != want {
		t.Errorf("seed = %s", data)
	}
}
