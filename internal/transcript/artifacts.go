package transcript

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	artifactCallMark = []byte(`"name":"Artifact"`)
	writeCallMark    = []byte(`"name":"Write"`)
	titleTagMark     = []byte(`<title>`)
	toolResultMark   = []byte(`"tool_result"`)
	listTitleRe      = regexp.MustCompile(`\\n- \([^)\\]{1,40}\) ([^\\]{1,160}?) \x{2014} (https://claude\.ai/(?:code/)?artifact/[A-Za-z0-9-]+)`)
	htmlTitleRe      = regexp.MustCompile(`(?is)<title>([^<]{1,200})</title>`)
)

type artifactCall struct {
	Title    string
	Explicit bool
	TypeURL  string
}

type toolRecord struct {
	Message struct {
		Content []struct {
			Type      string          `json:"type"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
			Input     struct {
				Action   string `json:"action"`
				TypeURL  string `json:"type_url"`
				Title    string `json:"title"`
				FilePath string `json:"file_path"`
				Content  string `json:"content"`
				Asset    bool   `json:"asset"`
			} `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

func noteToolCalls(e *entry, line []byte) {
	writes := bytes.Contains(line, writeCallMark) && bytes.Contains(line, titleTagMark)
	if !writes && !bytes.Contains(line, artifactCallMark) {
		return
	}
	var rec toolRecord
	if json.Unmarshal(line, &rec) != nil {
		return
	}
	for _, b := range rec.Message.Content {
		if b.Type != "tool_use" {
			continue
		}
		in := b.Input
		switch {
		case b.Name == "Write" && isHTML(in.FilePath):
			if t := htmlTitle([]byte(in.Content)); t != "" {
				if e.Pages == nil {
					e.Pages = map[string]string{}
				}
				e.Pages[in.FilePath] = t
			}
		case b.Name == "Artifact":
			if in.Asset || (in.Action != "" && in.Action != "publish") {
				continue
			}
			c := artifactCall{Title: in.Title, Explicit: in.Title != "", TypeURL: in.TypeURL}
			if c.Title == "" {
				c.Title = e.Pages[in.FilePath]
			}
			if c.Title == "" {
				c.Title = pageTitle(in.FilePath)
			}
			e.addCall(b.ID, c)
		}
	}
}

func (e *entry) addCall(id string, c artifactCall) {
	if e.Calls == nil {
		e.Calls = map[string]artifactCall{}
	}
	e.Calls[id] = c
}

func applyToolResults(e *entry, line []byte) {
	if len(e.Calls) == 0 || !bytes.Contains(line, toolResultMark) {
		return
	}
	pending := false
	for id := range e.Calls {
		if bytes.Contains(line, []byte(id)) {
			pending = true
			break
		}
	}
	if !pending {
		return
	}
	var rec toolRecord
	if json.Unmarshal(line, &rec) != nil {
		return
	}
	for _, b := range rec.Message.Content {
		c, ok := e.Calls[b.ToolUseID]
		if b.Type != "tool_result" || !ok {
			continue
		}
		delete(e.Calls, b.ToolUseID)
		for _, u := range artifactRe.FindAll(b.Content, -1) {
			if string(u) == c.TypeURL {
				continue
			}
			addArtifact(&e.Summary, string(u))
			if _, named := e.Summary.ArtifactTitles[string(u)]; c.Explicit || !named {
				setArtifactTitle(&e.Summary, string(u), c.Title)
			}
			break
		}
	}
}

func applyListTitles(e *entry, line []byte) {
	for _, m := range listTitleRe.FindAllSubmatch(line, -1) {
		setArtifactTitle(&e.Summary, string(m[2]), string(m[1]))
	}
}

func isHTML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".html" || ext == ".htm"
}

func pageTitle(path string) string {
	if !isHTML(path) {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	head, _ := io.ReadAll(io.LimitReader(f, 64<<10))
	return htmlTitle(head)
}

func htmlTitle(page []byte) string {
	if m := htmlTitleRe.FindSubmatch(page); m != nil {
		return strings.TrimSpace(html.UnescapeString(string(m[1])))
	}
	return ""
}

func setArtifactTitle(s *Summary, url, title string) {
	if title = strings.TrimSpace(title); title == "" {
		return
	}
	if s.ArtifactTitles == nil {
		s.ArtifactTitles = map[string]string{}
	}
	s.ArtifactTitles[url] = title
}
