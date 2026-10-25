package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/benteven/claude-deck/internal/deck"
	"github.com/benteven/claude-deck/internal/events"
	"github.com/benteven/claude-deck/internal/live"
	"github.com/benteven/claude-deck/internal/store"
	"github.com/benteven/claude-deck/internal/textclean"
	"github.com/benteven/claude-deck/internal/transcript"
)

type picker struct {
	input   textinput.Model
	folders []string
	shown   []string
	cursor  int
	path    string
	state   int
}

func existingFolders(summaries map[string]transcript.Summary) []string {
	sums := make([]transcript.Summary, 0, len(summaries))
	for _, s := range summaries {
		if s.Cwd != "" {
			sums = append(sums, s)
		}
	}
	sort.Slice(sums, func(i, j int) bool { return sums[i].LastUsed.After(sums[j].LastUsed) })
	seen := map[string]bool{}
	folders := []string{}
	for _, s := range sums {
		if seen[s.Cwd] {
			continue
		}
		seen[s.Cwd] = true
		if info, err := os.Stat(s.Cwd); err == nil && info.IsDir() {
			folders = append(folders, s.Cwd)
		}
	}
	return folders
}

func newPicker(folders []string, width int) *picker {
	p := &picker{input: textinput.New(), folders: folders}
	p.input.Prompt = "> "
	p.input.Placeholder = "filter folders"
	p.input.SetWidth(max(width-4, 1))
	p.filter()
	return p
}

func (p *picker) filter() {
	terms := strings.Fields(strings.ToLower(p.input.Value()))
	p.shown = p.shown[:0]
	p.path, p.state = resolveTyped(p.input.Value())
	if p.state == pathOK {
		p.shown = append(p.shown, p.path)
	}
	for _, f := range p.folders {
		if p.state == pathOK && f == p.path {
			continue
		}
		hay := strings.ToLower(f)
		match := true
		for _, t := range terms {
			if !strings.Contains(hay, t) {
				match = false
				break
			}
		}
		if match {
			p.shown = append(p.shown, f)
		}
	}
	p.cursor = min(p.cursor, max(len(p.shown)-1, 0))
}

func (m *model) pickerKey(msg tea.KeyPressMsg) tea.Cmd {
	p := m.picker
	switch msg.String() {
	case "esc":
		m.picker = nil
		return m.zoom(false)
	case "enter":
		if len(p.shown) == 0 || p.state == pathMissing {
			return nil
		}
		m.picker = nil
		return m.start(p.shown[p.cursor])
	case "up":
		p.cursor = max(p.cursor-1, 0)
		return nil
	case "down":
		p.cursor = min(p.cursor+1, max(len(p.shown)-1, 0))
		return nil
	case "tab":
		return complete(p.input.Value())
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.filter()
	if p.state == pathOK {
		p.cursor = 0
	}
	return cmd
}

func (p *picker) view(w, h int, focused bool) string {
	body := []string{white.Render("New session"), p.input.View(), ""}
	room := max(h-1-len(body), 1)
	start := max(p.cursor-room+1, 0)
	home, _ := os.UserHomeDir()
	for i := start; i < len(p.shown) && i < start+room; i++ {
		f := textclean.Clean(p.shown[i])
		if home != "" && (f == home || strings.HasPrefix(f, home+"/")) {
			f = "~" + f[len(home):]
		}
		if i == 0 && p.state == pathOK {
			f = "Start in " + f
		}
		body = append(body, ansi.Truncate(selectionBar(i == p.cursor, focused)+f, w, "…"))
	}
	switch {
	case p.state == pathMissing:
		body = append(body, gray.Render("no such folder"))
	case len(p.shown) == 0:
		body = append(body, gray.Render("No folders match."))
	}
	hint := gray.Render("↑↓ move  tab complete") + "\n" + gray.Render("enter start  esc cancel")
	return page(body, hint, h)
}

type renameDialog struct {
	input    textinput.Model
	id, pane string
	name     string
	suspend  bool
	busy     bool
	fresh    bool
}

func newRenameDialog(r deck.Row, suspend bool, width int) *renameDialog {
	d := &renameDialog{input: textinput.New(), id: r.SessionID, pane: r.PaneID, name: r.Name, suspend: suspend, busy: r.Busy, fresh: true}
	d.input.Prompt = "> "
	d.input.SetValue(r.Name)
	d.input.CursorEnd()
	d.input.SetWidth(max(width-4, 1))
	return d
}

func (m *model) renameKey(msg tea.KeyPressMsg) tea.Cmd {
	d := m.rename
	switch msg.String() {
	case "esc":
		m.rename = nil
		return m.zoom(false)
	case "enter":
		m.rename = nil
		return m.confirmRename(d)
	}
	if d.fresh && msg.Text != "" {
		d.input.SetValue("")
	}
	d.fresh = false
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(msg)
	return cmd
}

func (d *renameDialog) view(w, h int) string {
	verb := "Rename"
	if d.suspend {
		verb = "Suspend"
	}
	title := white.Render(ansi.Truncate(verb+` "`+d.name+`"`, w, "…"))
	body := []string{title, gray.Render(ansi.Truncate("Rename so you find it again (enter keeps it)", w, "…")), d.input.View()}
	if d.suspend && d.busy {
		body = append(body, "", yellow.Render(ansi.Truncate("This session is busy. Suspending stops the turn in progress.", w, "…")))
	}
	return page(body, gray.Render(ansi.Truncate("enter confirm  esc cancel", w, "")), h)
}

type resumePrompt struct{ count int }

func (p *resumePrompt) view(w, h int) string {
	body := []string{white.Render(ansi.Truncate(fmt.Sprintf("Resume %s from before? (y/n)", sessionCount(p.count)), w, "…"))}
	return page(body, gray.Render(ansi.Truncate("y resume all  n keep them gray", w, "")), h)
}

func sessionCount(n int) string {
	if n == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", n)
}

type takeoverDialog struct{ row deck.Row }

func (d *takeoverDialog) view(w, h int) string {
	body := []string{
		white.Render(ansi.Truncate(`Take over "`+d.row.Name+`"`, w, "…")),
		gray.Render(ansi.Truncate("Stop it in the other terminal and open it here? (y/n)", w, "…")),
	}
	if d.row.Busy {
		body = append(body, "", yellow.Render(ansi.Truncate("This session is busy. Taking over stops the turn in progress.", w, "…")))
	}
	return page(body, gray.Render(ansi.Truncate("y take over  n cancel", w, "")), h)
}

func (m *model) takeoverKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "y":
		r := m.takeover.row
		m.takeover = nil
		if m.resuming[r.SessionID] {
			return m.zoom(false)
		}
		m.resuming[r.SessionID] = true
		m.markEnding(r.SessionID)
		m.rebuild()
		ops := m.ops
		stop := func() tea.Msg {
			return takenOverMsg{row: r, err: ops.StopOutside(r.PID, r.Started, 5*time.Second)}
		}
		return tea.Sequence(m.zoom(false), m.showSelected(), stop)
	case "n", "esc":
		m.takeover = nil
		return m.zoom(false)
	}
	return nil
}

func (m *model) tookOver(msg takenOverMsg) tea.Cmd {
	delete(m.ending, msg.row.SessionID)
	if msg.err != nil {
		delete(m.resuming, msg.row.SessionID)
		m.err = msg.err
		m.rebuild()
		return nil
	}
	return m.resume(msg.row)
}

type quitDialog struct{ count int }

func (d *quitDialog) view(w, h int) string {
	body := []string{white.Render(ansi.Truncate(fmt.Sprintf("Stop %s and quit? (y/n)", sessionCount(d.count)), w, "…"))}
	return page(body, gray.Render(ansi.Truncate("y stop and quit  n cancel", w, "")), h)
}

func (m *model) quitKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "y":
		m.quit = nil
		m.quitting = true
		return m.stopAll()
	case "n", "esc":
		m.quit = nil
		return m.zoom(false)
	}
	return nil
}

type deleteDialog struct{ row deck.Row }

type trashedMsg struct {
	id  string
	err error
}

func (d *deleteDialog) view(w, h int) string {
	body := []string{
		white.Render(ansi.Truncate(`Delete "`+d.row.Name+`"? (y/n)`, w, "…")),
		gray.Render(ansi.Truncate("Moves it to the Trash.", w, "…")),
	}
	return page(body, gray.Render(ansi.Truncate("y delete  n cancel", w, "")), h)
}

func (m *model) deleteKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "y":
		id := m.del.row.SessionID
		m.del = nil
		i := slices.IndexFunc(m.suspended, func(r deck.Row) bool { return r.SessionID == id })
		live := slices.ContainsFunc(m.in.Live, func(l live.Session) bool { return l.SessionID == id })
		active := slices.ContainsFunc(m.state.Active, func(e store.ActiveEntry) bool { return e.SessionID == id })
		if i < 0 || live || active || m.resuming[id] || m.ending[id] {
			m.hint = "not suspended any more, not deleted"
			return m.zoom(false)
		}
		return tea.Sequence(m.zoom(false), m.trash(m.suspended[i]))
	case "n", "esc":
		m.del = nil
		return m.zoom(false)
	}
	return nil
}

func (m *model) trash(r deck.Row) tea.Cmd {
	eventsDir := filepath.Join(m.home, "events")
	return func() tea.Msg {
		if err := deck.TrashSession(r.SessionID, r.TranscriptPath); err != nil {
			return errMsg{err}
		}
		return trashedMsg{id: r.SessionID, err: events.Remove(eventsDir, r.SessionID)}
	}
}

func (m *model) trashed(msg trashedMsg) tea.Cmd {
	if m.gone == nil {
		m.gone = map[string]bool{}
	}
	m.gone[msg.id] = true
	m.err = msg.err
	delete(m.state.Names, msg.id)
	delete(m.in.Summaries, msg.id)
	m.in.Files = m.withoutGone(m.in.Files)
	m.rebuild()
	return tea.Batch(m.saveState(), m.showSelected())
}

func (m *model) withoutGone(files []transcript.File) []transcript.File {
	if len(m.gone) == 0 {
		return files
	}
	out := make([]transcript.File, 0, len(files))
	for _, f := range files {
		if !m.gone[f.SessionID] {
			out = append(out, f)
		}
	}
	return out
}

type restartDialog struct{ row deck.Row }

type restartedMsg struct {
	row  deck.Row
	pane string
	err  error

	stopFailed bool
}

func (d *restartDialog) view(w, h int) string {
	body := []string{white.Render(ansi.Truncate(`Restart "`+d.row.Name+`"? (y/n)`, w, "…"))}
	if d.row.Busy {
		body = append(body, "", yellow.Render(ansi.Truncate("This session is busy. Restarting stops the turn in progress.", w, "…")))
	}
	return page(body, gray.Render(ansi.Truncate("y restart  n cancel", w, "")), h)
}

func (m *model) restartKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "y":
		r := m.restart.row
		m.restart = nil
		if m.resuming[r.SessionID] {
			return m.zoom(false)
		}
		m.resuming[r.SessionID] = true
		m.markEnding(r.SessionID)
		delete(m.in.Panes, r.SessionID)
		m.rebuild()
		ops := m.ops
		run := func() tea.Msg {
			if err := ops.Stop(r.PaneID, r.Busy); err != nil {
				return restartedMsg{row: r, err: err, stopFailed: true}
			}
			pane, err := ops.Start(r.Cwd, r.SessionID, r.Name, true)
			return restartedMsg{row: r, pane: pane, err: err}
		}
		return tea.Sequence(m.zoom(false), m.showSelected(), run)
	case "n", "esc":
		m.restart = nil
		return m.zoom(false)
	}
	return nil
}

func (m *model) restarted(msg restartedMsg) tea.Cmd {
	id := msg.row.SessionID
	delete(m.resuming, id)
	delete(m.ending, id)
	if msg.err != nil {
		m.err = msg.err
		if m.in.Panes == nil {
			m.in.Panes = map[string]string{}
		}
		if msg.stopFailed {
			m.in.Panes[id] = msg.row.PaneID
		}
		m.rebuild()
		return tea.Batch(m.showSelected(), m.refresh())
	}
	if m.in.Panes == nil {
		m.in.Panes = map[string]string{}
	}
	m.in.Panes[id] = msg.pane
	m.rebuild()
	return m.showSelected()
}
