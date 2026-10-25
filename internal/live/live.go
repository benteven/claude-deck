package live

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
)

type Session struct {
	PID        int    `json:"pid"`
	SessionID  string `json:"sessionId"`
	Cwd        string `json:"cwd"`
	Name       string `json:"name"`
	NameSource string `json:"nameSource"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"startedAt"`
}

func Running(claudeHome string) ([]Session, error) {
	files, err := filepath.Glob(filepath.Join(claudeHome, "sessions", "*.json"))
	if err != nil {
		return nil, err
	}
	var out []Session
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s Session
		if json.Unmarshal(data, &s) != nil || s.PID <= 0 || s.SessionID == "" {
			continue
		}
		if err := syscall.Kill(s.PID, 0); err != nil && err != syscall.EPERM {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}
