package deck

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benteven/claude-deck/internal/events"
	"github.com/benteven/claude-deck/internal/store"
	"github.com/benteven/claude-deck/internal/transcript"
)

func ids(rows []Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.SessionID
	}
	return out
}

func TestActiveOrder(t *testing.T) {
	t0 := time.Now()
	in := Inputs{
		State: store.State{Active: []store.ActiveEntry{
			{SessionID: "a", LastInteraction: t0},
			{SessionID: "b", LastInteraction: t0.Add(-time.Hour)},
			{SessionID: "c", LastInteraction: t0.Add(-2 * time.Hour)},
		}},
		Panes:     map[string]string{"a": "%1", "b": "%2", "c": "%3"},
		Events:    map[string]events.State{"b": {Event: "UserPromptSubmit", PromptAt: t0.Add(time.Minute)}},
		Summaries: map[string]transcript.Summary{"c": {LastUsed: t0.Add(2 * time.Minute)}},
	}
	active, _ := Build(in)
	assert.Equal(t, []string{"c", "b", "a"}, ids(active))
}

func TestStaleRows(t *testing.T) {
	in := Inputs{
		State: store.State{
			Names:  map[string]string{"gone": "ledger import fix"},
			Active: []store.ActiveEntry{{SessionID: "gone", Cwd: "/x"}},
		},
		Files: []transcript.File{{SessionID: "old", Path: "/p/old.jsonl"}},
	}
	active, suspended := Build(in)
	require.Len(t, active, 1)
	assert.Equal(t, Stale, active[0].Kind)
	assert.Equal(t, "ledger import fix", active[0].Name)
	assert.Equal(t, []string{"old"}, ids(suspended))
}

func TestFilter(t *testing.T) {
	rows := []Row{
		{SessionID: "1", Name: "ledger import fix", Summary: transcript.Summary{PRs: []transcript.PRLink{{Number: 4598}}}},
		{SessionID: "2", Name: "ledger csv", Cwd: "/Users/dev/Projects/shop"},
		{SessionID: "3", Name: "risk", Summary: transcript.Summary{Branch: "ledger-tests"}},
	}
	assert.Equal(t, []string{"1", "2", "3"}, ids(Filter(rows, "Ledger")))
	assert.Equal(t, []string{"2"}, ids(Filter(rows, "ledger shop")))
	assert.Equal(t, []string{"1"}, ids(Filter(rows, "#4598")))
}
