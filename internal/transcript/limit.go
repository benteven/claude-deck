package transcript

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"time"
)

type Limit struct {
	At     time.Time
	Resets time.Time
	Text   string
}

const unknownReset = 5 * time.Hour

var (
	rateLimitMark = []byte(`"error":"rate_limit"`)
	syntheticMark = []byte(`"model":"<synthetic>"`)
	resetsRe      = regexp.MustCompile(`resets (?:([A-Z][a-z]{2}) (\d{1,2}) at )?(\d{1,2})(?::(\d{2}))?(am|pm)(?: \(([^)]+)\))?`)
)

func noteLimit(e *entry, line []byte) {
	if !bytes.Contains(line, assistantMark) {
		return
	}
	if !bytes.Contains(line, syntheticMark) {
		e.Summary.Limit = Limit{}
		return
	}
	if !bytes.Contains(line, rateLimitMark) {
		return
	}
	var rec struct {
		Timestamp string `json:"timestamp"`
		Message   struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &rec) != nil {
		return
	}
	at, err := time.Parse(time.RFC3339, rec.Timestamp)
	if err != nil {
		return
	}
	text := ""
	for _, en := range assistantEntries(rec.Message.Content) {
		if en.Kind == Reply {
			text = en.Text
			break
		}
	}
	e.Summary.Limit = Limit{At: at, Resets: resetTime(text, at), Text: text}
}

func resetTime(text string, at time.Time) time.Time {
	m := resetsRe.FindStringSubmatch(text)
	if m == nil {
		return at.Add(unknownReset)
	}
	loc := time.Local
	if m[6] != "" {
		if l, err := time.LoadLocation(m[6]); err == nil {
			loc = l
		}
	}
	hour, _ := strconv.Atoi(m[3])
	minute, _ := strconv.Atoi(m[4])
	hour %= 12
	if m[5] == "pm" {
		hour += 12
	}
	local := at.In(loc)
	if m[1] != "" {
		month, err := time.Parse("Jan", m[1])
		if err != nil {
			return at.Add(unknownReset)
		}
		day, _ := strconv.Atoi(m[2])
		t := time.Date(local.Year(), month.Month(), day, hour, minute, 0, 0, loc)
		if t.Before(local) {
			t = t.AddDate(1, 0, 0)
		}
		return t
	}
	t := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !t.After(local) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}
