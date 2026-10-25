package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Config struct {
	Prefix     string   `json:"prefix"`
	ClaudeArgs []string `json:"claude_args"`
}

type ActiveEntry struct {
	SessionID       string    `json:"session_id"`
	Cwd             string    `json:"cwd"`
	Name            string    `json:"name"`
	LastInteraction time.Time `json:"last_interaction"`
	Num             int       `json:"num"`
}

type State struct {
	Names  map[string]string `json:"names"`
	Active []ActiveEntry     `json:"active"`
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func LoadConfig(dir string) (Config, error) {
	c := Config{Prefix: "C-b"}
	if err := readJSON(filepath.Join(dir, "config.json"), &c); err != nil {
		return Config{Prefix: "C-b"}, err
	}
	if c.Prefix == "" || strings.EqualFold(c.Prefix, "C-j") || strings.EqualFold(c.Prefix, "C-k") {
		c.Prefix = "C-b"
	}
	return c, nil
}

func Load(dir string) (State, error) {
	var s State
	if err := readJSON(filepath.Join(dir, "state.json"), &s); err != nil {
		return State{}, err
	}
	if s.Names == nil {
		s.Names = map[string]string{}
	}
	for i := range s.Active {
		if s.Active[i].Num == 0 {
			s.Active[i].Num = Claim(s.Active, 0)
		}
	}
	return s, nil
}

func Claim(active []ActiveEntry, want int) int {
	taken := func(n int) bool {
		return slices.ContainsFunc(active, func(e ActiveEntry) bool { return e.Num == n })
	}
	if want > 0 && !taken(want) {
		return want
	}
	for n := 1; n <= 9; n++ {
		if !taken(n) {
			return n
		}
	}
	return 0
}

func (s State) Save(dir string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, "state.json"))
}
