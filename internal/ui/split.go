package ui

import (
	"slices"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/benteven/deckhand/internal/accounts"
	"github.com/benteven/deckhand/internal/deck"
)

func (m *model) group(r deck.Row) (string, bool) {
	switch {
	case r.Kind == deck.Live && r.PaneID != "":
		if a, ok := m.paneAccount[r.PaneID]; ok {
			return a, true
		}
		return m.state.AccountFor(r.SessionID), true
	case r.Kind == deck.Live || r.Kind == deck.Stale:
		return m.state.AccountFor(r.SessionID), true
	}
	return "", false
}

func (m *model) split() bool {
	seen := map[string]bool{}
	for _, r := range m.active {
		if a, ok := m.group(r); ok {
			seen[a] = true
		}
	}
	return len(seen) > 1
}

func (m *model) groupActive() {
	if !m.split() {
		return
	}
	rank := func(r deck.Row) (int, string) {
		a, ok := m.group(r)
		switch {
		case !ok:
			return 2, ""
		case a == "":
			return 0, ""
		}
		return 1, a
	}
	sort.SliceStable(m.active, func(i, j int) bool {
		ri, ai := rank(m.active[i])
		rj, aj := rank(m.active[j])
		if ri != rj {
			return ri < rj
		}
		return ai < aj
	})
}

func (m *model) rowHeader(rows []deck.Row, i int) string {
	if m.mode != modeActive {
		return ""
	}
	r := rows[i]
	if r.Kind == deck.Elsewhere {
		if i == 0 || rows[i-1].Kind != deck.Elsewhere {
			return "Elsewhere"
		}
		return ""
	}
	if !m.split() {
		return ""
	}
	a, _ := m.group(r)
	if i > 0 {
		if prev, ok := m.group(rows[i-1]); ok && prev == a {
			return ""
		}
	}
	label := accounts.Label(a)
	if k := accounts.Index(m.accountList, a); k >= 0 && m.accountList[k].Email != "" {
		label += " · " + m.accountList[k].Email
	}
	return label
}

func (m *model) loggedIn() []string {
	var out []string
	for _, a := range m.accountList {
		if a.Name == "" || a.Email != "" {
			out = append(out, a.Name)
		}
	}
	return out
}

func nextAccount(list []string, cur string) string {
	return list[(slices.Index(list, cur)+1)%len(list)]
}

func (m *model) moveAccount(r deck.Row) tea.Cmd {
	list := m.loggedIn()
	if len(list) < 2 {
		m.hint = "Add a second account with A first"
		return nil
	}
	cur, ok := m.group(r)
	if !ok {
		cur = m.state.AccountFor(r.SessionID)
	}
	return m.switchOne(r, nextAccount(list, cur))
}

func (m *model) cursorAccount() string {
	if r, ok := m.current(); ok && m.mode == modeActive {
		if a, ok := m.group(r); ok {
			return a
		}
	}
	return m.state.Account
}
