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
	typeURLMark      = []byte(`type_url`)
	typeURLRe        = regexp.MustCompile(`type_url[^h]{0,8}(https://claude\.ai/(?:code/)?artifact/[A-Za-z0-9-]+)`)
	listTitleRe      = regexp.MustCompile(`\\n- \([^)\\]{1,40}\) ([^\\]{1,160}?) \x{2014} (https://claude\.ai/(?:code/)?artifact/[A-Za-z0-9-]+)`)
	htmlTitleRe      = regexp.MustCompile(`(?is)<title>([^<]{1,200})</title>`)
)

type artifactCall struct {
	Title    string
	Explicit bool
	List     bool
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
		case b.Name == "Artifact" && in.Action == "list":
			e.addCall(b.ID, artifactCall{List: true})
		case b.Name == "Artifact" && !in.Asset && (in.Action == "" || in.Action == "publish"):
			c := artifactCall{Title: in.Title, Explicit: in.Title != ""}
			if c.Title == "" {
				c.Title = e.Pages[in.FilePath]
			}
			if c.Title == "" {
				c.Title = pageTitle(in.FilePath)
			}
			if c.Title != "" {
				e.addCall(b.ID, c)
			}
		}
	}
}

func (e *entry) addCall(id string, c artifactCall) {
	if e.Calls == nil {
		e.Calls = map[string]artifactCall{}
	}
	e.Calls[id] = c
}

func applyToolResults(e *entry, line []byte) (listed bool) {
	if len(e.Calls) == 0 || !bytes.Contains(line, toolResultMark) {
		return false
	}
	pending := false
	for id := range e.Calls {
		if bytes.Contains(line, []byte(id)) {
			pending = true
			break
		}
	}
	if !pending {
		return false
	}
	var rec toolRecord
	if json.Unmarshal(line, &rec) != nil {
		return false
	}
	for _, b := range rec.Message.Content {
		c, ok := e.Calls[b.ToolUseID]
		if b.Type != "tool_result" || !ok {
			continue
		}
		delete(e.Calls, b.ToolUseID)
		if c.List {
			listed = true
			continue
		}
		for _, u := range artifactRe.FindAll(b.Content, -1) {
			if e.Types[string(u)] {
				continue
			}
			if _, named := e.Summary.ArtifactTitles[string(u)]; c.Explicit || !named {
				setArtifactTitle(&e.Summary, string(u), c.Title)
			}
			break
		}
	}
	return listed
}

func applyArtifactMentions(e *entry, line []byte, listed bool) {
	if bytes.Contains(line, typeURLMark) {
		for _, m := range typeURLRe.FindAllSubmatch(line, -1) {
			if e.Types == nil {
				e.Types = map[string]bool{}
			}
			e.Types[string(m[1])] = true
		}
	}
	for _, m := range listTitleRe.FindAllSubmatch(line, -1) {
		setArtifactTitle(&e.Summary, string(m[2]), string(m[1]))
	}
	if listed {
		return
	}
	for _, u := range artifactRe.FindAll(line, -1) {
		addArtifact(&e.Summary, string(u))
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

func withoutTypes(urls []string, types map[string]bool) []string {
	if len(types) == 0 {
		return urls
	}
	var out []string
	for _, u := range urls {
		if !types[u] {
			out = append(out, u)
		}
	}
	return out
}
