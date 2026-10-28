package live

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	BridgeID   string `json:"bridgeSessionId"`
	Kind       string `json:"kind"`
	ParkedJob  string `json:"parkedJobId"`
}

func ParkedJobs(sessions []Session) map[string]string {
	jobs := map[string]string{}
	for _, s := range sessions {
		if s.ParkedJob == "" {
			continue
		}
		for _, j := range sessions {
			if j.Kind == "bg" && strings.HasPrefix(j.SessionID, s.ParkedJob) {
				jobs[s.SessionID] = j.SessionID
			}
		}
	}
	return jobs
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
