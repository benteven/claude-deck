package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/benteven/claude-deck/internal/accounts"
	"github.com/benteven/claude-deck/internal/deck"
	"github.com/benteven/claude-deck/internal/tmux"
)

type accountDialog struct {
	row    deck.Row
	hasRow bool
	list   []accounts.Account
	cursor int
	naming bool
	input  textinput.Model
}

func (m *model) openAccounts() tea.Cmd {
	d := &accountDialog{list: accounts.List(m.home, m.claudeHome)}
	if r, ok := m.current(); ok && (r.Kind == deck.Live || r.Kind == deck.Suspended) {
		d.row, d.hasRow = r, true
	}
	current := m.state.Account
	if d.hasRow {
		current = m.rowAccount(d.row)
	}
	d.cursor = max(accounts.Index(d.list, current), 0)
	m.accounts = d
	return m.zoom(true)
}

func (m *model) rowAccount(r deck.Row) string {
	switch r.Kind {
	case deck.Live:
		return m.paneAccount[r.PaneID]
	case deck.Suspended:
		return m.state.AccountFor(r.SessionID)
	}
	return ""
}

func (m *model) accountTag(r deck.Row) string {
	cur := m.rowAccount(r)
	if r.Kind == deck.Live && m.switches[r.PaneID] {
		return accounts.Label(cur) + " → " + accounts.Label(m.state.AccountFor(r.SessionID))
	}
	return cur
}

func (m *model) accountsView(w, h int) string {
	d := m.accounts
	if d.naming {
		body := []string{
			white.Render(ansi.Truncate("Name for the new account:", w, "…")),
			"",
			ansi.Truncate(selectionBar(true, m.focused)+d.input.View(), w, ""),
			"",
			gray.Render(ansi.Truncate("Use a-z, 0-9, - and _. Then log in with that account.", w, "…")),
		}
		return page(body, gray.Render(ansi.Truncate("enter log in  esc cancel", w, "")), h)
	}
	title := "Accounts"
	if d.hasRow {
		title = `Account for "` + d.row.Name + `"`
	}
	body := []string{white.Render(ansi.Truncate(title, w, "…")), ""}
	col := 0
	for _, a := range d.list {
		col = max(col, ansi.StringWidth(accounts.Label(a.Name)))
	}
	for i, a := range d.list {
		label := accounts.Label(a.Name)
		email := gray.Render(a.Email)
		if a.Email == "" {
			email = yellow.Render("not logged in")
		}
		var tags []string
		if d.hasRow && a.Name == m.rowAccount(d.row) {
			tags = append(tags, "this session")
		}
		if a.Name == m.state.Account {
			tags = append(tags, "new sessions")
		}
		line := selectionBar(i == d.cursor, m.focused) + white.Render(label) + strings.Repeat(" ", col+2-ansi.StringWidth(label)) + email
		if len(tags) > 0 {
			line += "  " + orange.Render(strings.Join(tags, ", "))
		}
		body = append(body, ansi.Truncate(line, w, "…"))
	}
	body = append(body, ansi.Truncate(selectionBar(d.cursor == len(d.list), m.focused)+gray.Render("+ add account"), w, ""))
	hint := "enter all sessions  esc cancel"
	if d.hasRow {
		hint = "enter this session  A all sessions  esc cancel"
	}
	if d.cursor == len(d.list) {
		hint = "enter add  esc cancel"
	}
	return page(body, gray.Render(ansi.Truncate(hint, w, "")), h)
}

func (m *model) accountsKey(msg tea.KeyPressMsg) tea.Cmd {
	d := m.accounts
	if d.naming {
		switch msg.String() {
		case "esc":
			d.naming = false
			return nil
		case "enter":
			name := strings.TrimSpace(d.input.Value())
			switch {
			case !accounts.ValidName(name):
				m.hint = "Use a-z, 0-9, - and _ in the name"
				return nil
			case accounts.Exists(m.home, name):
				m.hint = name + " exists already"
				return nil
			}
			m.accounts = nil
			return tea.Sequence(m.zoom(false), m.login(name))
		}
		var cmd tea.Cmd
		d.input, cmd = d.input.Update(msg)
		return cmd
	}
	switch msg.String() {
	case "up", "k":
		d.cursor = max(d.cursor-1, 0)
	case "down", "j":
		d.cursor = min(d.cursor+1, len(d.list))
	case "esc", "q":
		m.accounts = nil
		return m.zoom(false)
	case "enter", "A":
		if d.cursor == len(d.list) {
			if msg.String() == "A" {
				return nil
			}
			d.naming = true
			d.input = textinput.New()
			d.input.Prompt = ""
			return d.input.Focus()
		}
		name := d.list[d.cursor].Name
		m.accounts = nil
		if msg.String() == "A" || !d.hasRow {
			return tea.Sequence(m.zoom(false), m.switchAll(name))
		}
		return tea.Sequence(m.zoom(false), m.switchOne(d.row, name))
	}
	return nil
}

func (m *model) switchOne(r deck.Row, name string) tea.Cmd {
	if m.state.Accounts == nil {
		m.state.Accounts = map[string]string{}
	}
	m.state.Accounts[r.SessionID] = name
	if r.Kind != deck.Live {
		m.hint = fmt.Sprintf("%s resumes with %s", r.Name, accounts.Label(name))
		return m.saveState()
	}
	return tea.Batch(m.saveState(), m.queueSwitch([]deck.Row{r}))
}

func (m *model) switchAll(name string) tea.Cmd {
	m.state.Account = name
	m.state.Accounts = nil
	return tea.Batch(m.saveState(), m.queueSwitch(m.liveRows()))
}

func (m *model) queueSwitch(rows []deck.Row) tea.Cmd {
	if m.switches == nil {
		m.switches = map[string]bool{}
	}
	n, busy := 0, 0
	for _, r := range rows {
		if m.paneAccount[r.PaneID] != m.state.AccountFor(r.SessionID) {
			m.switches[r.PaneID] = true
			n++
			if r.Busy {
				busy++
			}
		}
	}
	switch {
	case n == 0:
		m.hint = "Already on that account"
		return nil
	case busy > 0:
		m.hint = fmt.Sprintf("Switching %s, %d when idle", sessionCount(n), busy)
	default:
		m.hint = "Switching " + sessionCount(n)
	}
	return m.advanceSwitches()
}

func (m *model) advanceSwitches() tea.Cmd {
	var cmds []tea.Cmd
	for pane := range m.switches {
		r, ok := m.rowInPane(pane)
		switch {
		case !ok || m.paneAccount[pane] == m.state.AccountFor(r.SessionID):
			delete(m.switches, pane)
		case !r.Busy && !m.resuming[r.SessionID] && m.handoffs[pane] == nil:
			delete(m.switches, pane)
			cmds = append(cmds, m.restartRow(r))
		}
	}
	return tea.Batch(cmds...)
}

func (m *model) login(name string) tea.Cmd {
	exe, err := os.Executable()
	if err != nil {
		m.err = err
		return nil
	}
	ops := m.ops
	return func() tea.Msg {
		pane, err := ops.Login(exe, name)
		return loginMsg{pane: pane, name: name, err: err}
	}
}

type loginMsg struct {
	pane, name string
	err        error
}

func (m *model) loginStarted(msg loginMsg) tea.Cmd {
	if msg.err != nil {
		m.err = msg.err
		return nil
	}
	m.loginPane = msg.pane
	return tea.Sequence(m.show(msg.pane, "", false), m.focus(tmux.Slot))
}

func (m *model) endLogin(dead bool, gone bool) tea.Cmd {
	if m.loginPane == "" || (!dead && !gone) {
		return nil
	}
	pane := m.loginPane
	m.loginPane = ""
	m.hint = "Press A to pick the new account"
	cmds := []tea.Cmd{m.showSelected(), m.focus(tmux.ListPane)}
	if !gone {
		cmds = append(cmds, m.tmux("kill-pane", "-t", pane))
	}
	return tea.Sequence(cmds...)
}
