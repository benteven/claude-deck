package ui

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/benteven/deckhand/internal/deck"
	"github.com/benteven/deckhand/internal/tmux"
)

const cycleWindow = 1500 * time.Millisecond

type cycle struct {
	order []string
	i     int
	seq   int
	at    time.Time
}

type cycleDoneMsg struct{ seq int }

func (m *model) visit(id string) {
	if id == "" {
		return
	}
	m.mru = append([]string{id}, slices.DeleteFunc(m.mru, func(s string) bool { return s == id })...)
}

func (m *model) slotSession() string {
	for _, r := range m.liveRows() {
		if r.PaneID == m.slot {
			return r.SessionID
		}
	}
	return ""
}

func (m *model) recent() []string {
	rows := m.liveRows()
	live := func(id string) bool {
		return slices.ContainsFunc(rows, func(r deck.Row) bool { return r.SessionID == id })
	}
	var order []string
	add := func(id string) {
		if live(id) && !slices.Contains(order, id) {
			order = append(order, id)
		}
	}
	add(m.slotSession())
	add(m.selected)
	for _, id := range m.mru {
		add(id)
	}
	for _, r := range rows {
		add(r.SessionID)
	}
	return order
}

func (m *model) toggleLast() tea.Cmd {
	if m.quitting || m.dialogOpen() {
		return nil
	}
	now := time.Now()
	c := m.cycle
	if c == nil || now.Sub(c.at) > cycleWindow {
		c = &cycle{order: m.recent()}
		m.cycle = c
	}
	if len(c.order) < 2 {
		m.cycle = nil
		m.hint = "No other session to go back to"
		return nil
	}
	c.i = (c.i + 1) % len(c.order)
	c.at = now
	c.seq++
	i := slices.IndexFunc(m.liveRows(), func(r deck.Row) bool { return r.SessionID == c.order[c.i] })
	if i < 0 {
		return nil
	}
	r := m.liveRows()[i]
	m.selectRow(r.SessionID)
	seq := c.seq
	return tea.Batch(
		tea.Sequence(m.show(r.PaneID, "", false), m.focus(tmux.Slot)),
		tea.Tick(cycleWindow, func(time.Time) tea.Msg { return cycleDoneMsg{seq} }),
	)
}

func (m *model) cycleDone(msg cycleDoneMsg) {
	if c := m.cycle; c != nil && c.seq == msg.seq {
		m.visit(c.order[c.i])
		m.cycle = nil
	}
}

func (m *model) switcherView(w, h int) string {
	c := m.cycle
	rows := m.liveRows()
	body := []string{white.Underline(true).Render("Recent")}
	inner := max(w-2, 1)
	for k, id := range c.order {
		i := slices.IndexFunc(rows, func(r deck.Row) bool { return r.SessionID == id })
		if i < 0 {
			continue
		}
		r := rows[i]
		if k == c.i {
			body = append(body, rowLines(r, w, true, true, -1, m.prState, m.accountTag(r), m.nameStyle(r))...)
			continue
		}
		body = append(body, ansi.Truncate(selectionBar(false, true)+nameLine(r.Name, m.nameStyle(r), "", r.Num, r.Remote, inner), w, ""))
	}
	return page(body, gray.Render(ansi.Truncate("ctrl-q next, wait to pick", w, "")), h)
}
