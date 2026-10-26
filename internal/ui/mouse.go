package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/benteven/claude-deck/internal/transcript"
)

func (m *model) mouseActive() bool {
	return !m.dialogOpen() && !m.quitting
}

func (m *model) wheel(ms tea.Mouse) tea.Cmd {
	if !m.mouseActive() {
		return nil
	}
	switch ms.Button {
	case tea.MouseWheelUp:
		return m.move(-1)
	case tea.MouseWheelDown:
		return m.move(1)
	}
	return nil
}

func (m *model) click(ms tea.Mouse) tea.Cmd {
	if m.rename != nil {
		if ms.Button == tea.MouseLeft {
			if row, _, ok := m.hitAt(ms.X, ms.Y); !ok || row != m.cursor {
				m.rename = nil
			}
		}
		return nil
	}
	if !m.mouseActive() || ms.Button != tea.MouseLeft {
		return nil
	}
	if ms.Y == 0 {
		return m.clickTab(ms.X)
	}
	row, url, ok := m.hitAt(ms.X, ms.Y)
	if !ok {
		return nil
	}
	m.hint = ""
	m.err = nil
	if url != "" {
		return openURLs([]string{url})
	}
	return m.selectIndex(row)
}

func (m *model) clickTab(x int) tea.Cmd {
	a, s := m.tabLabels()
	aw := ansi.StringWidth(a)
	next := modeActive
	switch {
	case x >= aw+2 && x < aw+2+ansi.StringWidth(s):
		next = modeSuspended
	case x >= aw:
		return nil
	}
	if !m.setMode(next) {
		return nil
	}
	m.hint = ""
	m.err = nil
	return m.showSelected()
}

func (m *model) hitAt(x, y int) (row int, url string, ok bool) {
	w, h := m.size()
	blocks, _, room := m.layout(w, h)
	top := 1
	if m.searching || m.query != "" {
		top++
	}
	if y < top || y-top >= room {
		return 0, "", false
	}
	rows := m.shown()
	start := windowStart(blocks, m.cursor, room)
	line := top
	for i := start; i < len(blocks); i++ {
		if i > start {
			line++
		}
		for _, text := range blocks[i] {
			if line == y {
				return i, m.linkAt(rows[i].Summary.PRs, text, x), true
			}
			line++
		}
	}
	return 0, "", false
}

func (m *model) linkAt(prs []transcript.PRLink, line string, x int) string {
	for _, pr := range prs {
		open := strings.Index(line, ansi.SetHyperlink(pr.URL))
		if open < 0 {
			continue
		}
		from := ansi.StringWidth(line[:open])
		if prIcon(m.prState(pr.URL)) != "" {
			from -= 2
		}
		to := ansi.StringWidth(line)
		if end := strings.Index(line[open:], ansi.ResetHyperlink()); end >= 0 {
			to = ansi.StringWidth(line[:open+end])
		}
		if x >= from && x < to {
			return pr.URL
		}
	}
	return ""
}
