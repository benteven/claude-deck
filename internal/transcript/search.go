package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

const snippetRunes = 80

var userMark = []byte(`"type":"user"`)

type Index struct {
	mu   sync.Mutex
	docs map[string]*doc
}

type doc struct {
	size   int64
	offset int64
	text   strings.Builder
	lower  strings.Builder
}

func NewIndex() *Index {
	return &Index{docs: map[string]*doc{}}
}

func (x *Index) Search(path string, terms []string) (string, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	d, err := x.update(path)
	if err != nil || d.lower.Len() == 0 || len(terms) == 0 {
		return "", false
	}
	lower := d.lower.String()
	first := -1
	for _, t := range terms {
		i := strings.Index(lower, t)
		if i < 0 {
			return "", false
		}
		if first < 0 {
			first = i
		}
	}
	return snippet(d.text.String(), first), true
}

func (x *Index) update(path string) (*doc, error) {
	info, err := os.Stat(path)
	if err != nil {
		delete(x.docs, path)
		return nil, err
	}
	d := x.docs[path]
	if d == nil || info.Size() < d.size {
		d = &doc{}
		x.docs[path] = d
	}
	d.size = info.Size()
	if d.offset >= d.size {
		return d, nil
	}
	return d, d.read(path)
}

func (d *doc) read(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(d.offset, io.SeekStart); err != nil {
		return err
	}
	br := bufio.NewReaderSize(f, 1<<20)
	var pending []byte
	for {
		chunk, err := br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			pending = append(pending, chunk...)
			continue
		}
		line := chunk
		if pending != nil {
			line = append(pending, chunk...)
			pending = nil
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		d.offset += int64(len(line))
		d.add(line)
	}
}

func (d *doc) add(line []byte) {
	var texts []string
	switch {
	case bytes.Contains(line, assistantMark):
		var rec tailRecord
		if json.Unmarshal(line, &rec) != nil || rec.Type != "assistant" {
			return
		}
		for _, e := range assistantEntries(rec.Message.Content) {
			if e.Kind == Reply {
				texts = append(texts, e.Text)
			}
		}
	case bytes.Contains(line, userMark) && !bytes.Contains(line, toolResultMark):
		var rec tailRecord
		if json.Unmarshal(line, &rec) != nil || rec.Type != "user" || rec.IsMeta {
			return
		}
		texts = append(texts, userText(rec.Message.Content))
	}
	for _, t := range texts {
		if t = strings.Join(strings.Fields(t), " "); t == "" {
			continue
		}
		low := lowerSameLength(t)
		d.text.WriteString(t)
		d.text.WriteByte('\n')
		d.lower.WriteString(low)
		d.lower.WriteByte('\n')
	}
}

func lowerSameLength(s string) string {
	low := strings.ToLower(s)
	if len(low) == len(s) {
		return low
	}
	b := []byte(s)
	for i := range b {
		if 'A' <= b[i] && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func snippet(text string, at int) string {
	start := strings.LastIndexByte(text[:at], '\n') + 1
	end := strings.IndexByte(text[at:], '\n')
	if end < 0 {
		end = len(text)
	} else {
		end += at
	}
	line := text[start:end]
	at -= start
	if before := utf8.RuneCountInString(line[:at]); before > snippetRunes/4 {
		skip := before - snippetRunes/4
		i := 0
		for n := 0; n < skip; n++ {
			_, size := utf8.DecodeRuneInString(line[i:])
			i += size
		}
		line = "…" + line[i:]
	}
	return line
}
