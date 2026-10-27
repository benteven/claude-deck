package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/benteven/deckhand/internal/store"
	"github.com/benteven/deckhand/internal/tmux"
)

const widthStep = 4

type geometry struct {
	pane, option, window int
	zoomed               bool
}

func sampleGeometry(t tmux.Client) (geometry, bool) {
	out, err := t.Run("display-message", "-p", "-t", tmux.ListPane, "#{pane_width} #{"+tmux.WidthOption+"} #{window_width} #{window_zoomed_flag}")
	if err != nil {
		return geometry{}, false
	}
	f := strings.Fields(out)
	if len(f) != 4 {
		return geometry{}, false
	}
	var g geometry
	var perr [3]error
	g.pane, perr[0] = strconv.Atoi(f[0])
	g.option, perr[1] = strconv.Atoi(f[1])
	g.window, perr[2] = strconv.Atoi(f[2])
	g.zoomed = f[3] == "1"
	for _, e := range perr {
		if e != nil {
			return geometry{}, false
		}
	}
	return g, true
}

func (m *model) dragged(g geometry, ok bool) tea.Cmd {
	prev, seen := m.windowWidth, m.dragSeen
	m.dragSeen = 0
	if !ok {
		return nil
	}
	m.windowWidth = g.window
	if g.option > 0 {
		m.ops.Cfg.ListWidth = store.ClampListWidth(g.option)
	}
	if g.zoomed || prev == 0 || g.window != prev || g.pane == g.option || g.option >= g.window-2 {
		return nil
	}
	m.dragSeen = g.pane
	if seen != g.pane {
		return nil
	}
	return m.setWidth(g.pane)
}

func (m *model) resizeList(delta int) tea.Cmd {
	return m.setWidth(m.ops.Cfg.ListWidth + delta)
}

func (m *model) setWidth(w int) tea.Cmd {
	w = store.ClampListWidth(w)
	changed := w != m.ops.Cfg.ListWidth
	m.ops.Cfg.ListWidth = w
	t, home := m.ops.T, m.home
	return func() tea.Msg {
		if err := t.SetListWidth(w); err != nil {
			return errMsg{err}
		}
		if !changed {
			return nil
		}
		if err := store.SaveListWidth(home, w); err != nil {
			return errMsg{fmt.Errorf("save list width: %w", err)}
		}
		return nil
	}
}
