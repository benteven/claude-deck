package transcript

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
)

type EntryKind int

const (
	Prompt EntryKind = iota
	Reply
	Tool
)

type Entry struct {
	Kind EntryKind
	Text string
}

const (
	tailBytes    = 1 << 20
	toolArgRunes = 60
)

var toolArgKeys = []string{"file_path", "command", "pattern", "url", "description"}

type tailBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type tailRecord struct {
	Type    string `json:"type"`
	IsMeta  bool   `json:"isMeta"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func Tail(path string, n int) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var start int64
	if info.Size() > tailBytes {
		start = info.Size() - tailBytes
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if start > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return nil, nil
		}
		data = data[i+1:]
	}
	var entries []Entry
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var rec tailRecord
		if json.Unmarshal(line, &rec) != nil || rec.IsMeta {
			continue
		}
		switch rec.Type {
		case "user":
			if text := userText(rec.Message.Content); text != "" {
				entries = append(entries, Entry{Kind: Prompt, Text: text})
			}
		case "assistant":
			entries = append(entries, assistantEntries(rec.Message.Content)...)
		}
	}
	if len(entries) > n {
		entries = entries[len(entries)-n:]
	}
	return entries, nil
}

func userText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var blocks []tailBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			parts = append(parts, strings.TrimSpace(b.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func assistantEntries(raw json.RawMessage) []Entry {
	var blocks []tailBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	var out []Entry
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if text := strings.TrimSpace(b.Text); text != "" {
				out = append(out, Entry{Kind: Reply, Text: text})
			}
		case "tool_use":
			out = append(out, Entry{Kind: Tool, Text: toolText(b)})
		}
	}
	return out
}

func toolText(b tailBlock) string {
	for _, key := range toolArgKeys {
		if v, ok := b.Input[key].(string); ok && v != "" {
			v = strings.Join(strings.Fields(v), " ")
			if r := []rune(v); len(r) > toolArgRunes {
				v = string(r[:toolArgRunes])
			}
			return b.Name + " " + v
		}
	}
	return b.Name
}
