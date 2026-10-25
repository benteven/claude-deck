package transcript

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type agentMeta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
	Finished    bool   `json:"finished"`
}

func agentsDir(transcriptPath string) string {
	if transcriptPath == "" {
		return ""
	}
	return filepath.Join(strings.TrimSuffix(transcriptPath, ".jsonl"), "subagents")
}

func readMeta(dir, id string) (agentMeta, error) {
	var m agentMeta
	data, err := os.ReadFile(filepath.Join(dir, "agent-"+id+".meta.json"))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(data, &m)
}

func (m agentMeta) label() string {
	return cmp.Or(m.Description, m.AgentType, "subagent")
}

func AgentDescriptions(transcriptPath string, ids []string) []string {
	dir := agentsDir(transcriptPath)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		var m agentMeta
		if dir != "" && id != "" && id == filepath.Base(id) {
			m, _ = readMeta(dir, id)
		}
		out = append(out, m.label())
	}
	return out
}

func RecentAgents(transcriptPath string, since time.Time) []string {
	dir := agentsDir(transcriptPath)
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, "agent-"), ".jsonl")
		info, err := e.Info()
		if err != nil || info.ModTime().Before(since) {
			continue
		}
		m, _ := readMeta(dir, id)
		if !m.Finished {
			out = append(out, m.label())
		}
	}
	return out
}
