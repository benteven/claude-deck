package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/benteven/deckhand/internal/deck"
)

type textHitsMsg struct {
	query string
	hits  map[string]string
}

type textHits struct {
	query string
	hits  map[string]string
}

func (m *model) searchText() tea.Cmd {
	if m.mode != modeSuspended {
		return nil
	}
	return m.indexText(m.query)
}

func (m *model) indexText(query string) tea.Cmd {
	index := m.index
	paths := map[string]string{}
	for _, r := range m.suspended {
		if r.TranscriptPath != "" {
			paths[r.SessionID] = r.TranscriptPath
		}
	}
	return func() tea.Msg {
		terms := strings.Fields(strings.ToLower(query))
		hits := map[string]string{}
		for id, path := range paths {
			if s, ok := index.Search(path, terms); ok {
				hits[id] = s
			}
		}
		return textHitsMsg{query: query, hits: hits}
	}
}

func (m *model) gotTextHits(msg textHitsMsg) tea.Cmd {
	if msg.query != m.query || msg.query == "" {
		return nil
	}
	m.text = textHits{query: msg.query, hits: msg.hits}
	keep := m.selected
	m.syncCursor()
	if m.selected == keep {
		return nil
	}
	return m.showSelected()
}

func (m *model) textPending() bool {
	return m.mode == modeSuspended && m.query != "" && m.text.query != m.query
}

func (m *model) textHit(id string) string {
	if m.mode != modeSuspended || m.text.query != m.query {
		return ""
	}
	return m.text.hits[id]
}

func (m *model) shownSuspended() []deck.Row {
	byName := deck.Filter(m.suspended, m.query)
	if m.query == "" || m.text.query != m.query {
		return byName
	}
	named := make(map[string]bool, len(byName))
	for _, r := range byName {
		named[r.SessionID] = true
	}
	var out []deck.Row
	for _, r := range m.suspended {
		if named[r.SessionID] || m.text.hits[r.SessionID] != "" {
			out = append(out, r)
		}
	}
	return out
}
