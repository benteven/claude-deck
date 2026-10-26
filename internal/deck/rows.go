package deck

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/benteven/claude-deck/internal/events"
	"github.com/benteven/claude-deck/internal/live"
	"github.com/benteven/claude-deck/internal/store"
	"github.com/benteven/claude-deck/internal/textclean"
	"github.com/benteven/claude-deck/internal/transcript"
)

type Kind int

const (
	Live Kind = iota
	Stale
	Elsewhere
	Suspended
)

type Row struct {
	SessionID       string
	Name            string
	Cwd             string
	PaneID          string
	PID             int
	Started         time.Time
	TranscriptPath  string
	Kind            Kind
	Summary         transcript.Summary
	Waiting         bool
	Busy            bool
	LastInteraction time.Time
	Agents          []string
	Num             int
}

type Inputs struct {
	State     store.State
	Panes     map[string]string
	Live      []live.Session
	Events    map[string]events.State
	Files     []transcript.File
	Summaries map[string]transcript.Summary
	Agents    map[string][]string
}

func Build(in Inputs) (active, suspended []Row) {
	liveByID := make(map[string]live.Session, len(in.Live))
	for _, s := range in.Live {
		liveByID[s.SessionID] = s
	}
	paths := make(map[string]string, len(in.Files))
	for _, f := range in.Files {
		paths[f.SessionID] = f.Path
	}
	seen := make(map[string]bool, len(in.State.Active))

	for _, e := range in.State.Active {
		seen[e.SessionID] = true
		sum := in.Summaries[e.SessionID]
		ls, isLive := liveByID[e.SessionID]
		ev := in.Events[e.SessionID]
		r := Row{
			SessionID:       e.SessionID,
			Name:            name(in.State, e.Name, ls, e.SessionID, sum),
			Cwd:             firstNonEmpty(e.Cwd, ls.Cwd, sum.Cwd),
			PaneID:          in.Panes[e.SessionID],
			TranscriptPath:  paths[e.SessionID],
			Kind:            Stale,
			Summary:         sum,
			Waiting:         ev.Waiting(),
			Busy:            isLive && ls.Status == "busy",
			LastInteraction: latest(e.LastInteraction, ev.PromptAt, sum.PromptedAt),
			Agents:          cleanAll(in.Agents[e.SessionID]),
			Num:             e.Num,
		}
		if r.PaneID != "" {
			r.Kind = Live
		}
		active = append(active, r)
	}

	for _, ls := range in.Live {
		if seen[ls.SessionID] {
			continue
		}
		seen[ls.SessionID] = true
		sum := in.Summaries[ls.SessionID]
		active = append(active, Row{
			SessionID:       ls.SessionID,
			Name:            name(in.State, "", ls, ls.SessionID, sum),
			Cwd:             firstNonEmpty(ls.Cwd, sum.Cwd),
			PID:             ls.PID,
			Started:         startTime(ls.StartedAt),
			TranscriptPath:  paths[ls.SessionID],
			Kind:            Elsewhere,
			Summary:         sum,
			Waiting:         ls.Status == "idle",
			Busy:            ls.Status == "busy",
			LastInteraction: latest(promptedAt(sum), in.Events[ls.SessionID].PromptAt),
			Agents:          cleanAll(in.Agents[ls.SessionID]),
		})
	}
	sort.SliceStable(active, func(i, j int) bool {
		if ei, ej := active[i].Kind == Elsewhere, active[j].Kind == Elsewhere; ei != ej {
			return ej
		}
		return active[i].LastInteraction.After(active[j].LastInteraction)
	})

	for _, f := range in.Files {
		if seen[f.SessionID] {
			continue
		}
		sum := in.Summaries[f.SessionID]
		last := sum.LastUsed
		if last.IsZero() {
			last = f.ModTime
		}
		suspended = append(suspended, Row{
			SessionID:       f.SessionID,
			Name:            name(in.State, "", live.Session{}, f.SessionID, sum),
			Cwd:             sum.Cwd,
			TranscriptPath:  f.Path,
			Kind:            Suspended,
			Summary:         sum,
			LastInteraction: last,
		})
	}
	sort.SliceStable(suspended, func(i, j int) bool {
		return suspended[i].LastInteraction.After(suspended[j].LastInteraction)
	})
	return active, suspended
}

func name(state store.State, entryName string, ls live.Session, id string, sum transcript.Summary) string {
	liveName := ""
	if ls.NameSource == "user" {
		liveName = ls.Name
	}
	var picked string
	for _, c := range []string{state.Names[id], entryName, liveName, sum.AITitle} {
		if clean(c) != "" {
			picked = c
			break
		}
	}
	if picked == "" {
		picked = cut(clean(sum.LastPrompt), 40)
	}
	if picked = clean(picked); picked != "" {
		return picked
	}
	return cut(id, 8)
}

func clean(s string) string {
	return strings.Join(strings.Fields(textclean.Clean(s)), " ")
}

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func startTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func promptedAt(sum transcript.Summary) time.Time {
	if sum.PromptedAt.IsZero() {
		return sum.LastUsed
	}
	return sum.PromptedAt
}

func latest(ts ...time.Time) time.Time {
	var out time.Time
	for _, t := range ts {
		if t.After(out) {
			out = t
		}
	}
	return out
}

func cleanAll(texts []string) []string {
	var out []string
	for _, t := range texts {
		if c := clean(t); c != "" {
			out = append(out, c)
		}
	}
	return out
}

func Filter(rows []Row, query string) []Row {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return rows
	}
	var out []Row
	for _, r := range rows {
		hay := haystack(r)
		match := true
		for _, t := range terms {
			if !strings.Contains(hay, t) {
				match = false
				break
			}
		}
		if match {
			out = append(out, r)
		}
	}
	return out
}

func haystack(r Row) string {
	parts := []string{r.Name, r.Summary.AITitle, r.Summary.LastPrompt, r.Cwd, r.Summary.Branch}
	for _, pr := range r.Summary.PRs {
		parts = append(parts, fmt.Sprintf("#%d", pr.Number))
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}
