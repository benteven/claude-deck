package transcript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prNumbers(prs []PRLink) []int {
	var out []int
	for _, p := range prs {
		out = append(out, p.Number)
	}
	return out
}

func copyFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/session.jsonl")
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "session.jsonl")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func appendRaw(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(text)
	require.NoError(t, err)
}

func TestSummary(t *testing.T) {
	r := NewReader(filepath.Join(t.TempDir(), "cache.json"))
	s, err := r.Summary("testdata/session.jsonl")
	require.NoError(t, err)
	assert.Equal(t, "Add a billing page", s.AITitle)
	assert.Equal(t, "billing", s.Title)
	assert.Equal(t, "develop", s.Branch)
	assert.Equal(t, "/Users/dev/Projects/shop", s.Cwd)
	assert.Equal(t, 205848, s.ContextTokens)
	assert.Equal(t, []int{4737, 4744}, prNumbers(s.PRs))
	assert.Equal(t, []string{"https://claude.ai/artifact/Billing1", "https://claude.ai/code/artifact/abc-123"}, s.Artifacts)
	assert.Equal(t, "Billing mockup", s.ArtifactTitles["https://claude.ai/artifact/Billing1"])
	assert.Equal(t, 20, s.ContextPercent())
}

func TestSummaryIncremental(t *testing.T) {
	path := copyFixture(t)
	cache := filepath.Join(t.TempDir(), "cache.json")
	r := NewReader(cache)
	_, err := r.Summary(path)
	require.NoError(t, err)
	appendRaw(t, path, `{"type":"ai-title","aiTitle":"Renamed`)
	s, _ := r.Summary(path)
	assert.Equal(t, "Add a billing page", s.AITitle)
	appendRaw(t, path, `"}`+"\n")
	require.NoError(t, r.Save())
	s, _ = NewReader(cache).Summary(path)
	assert.Equal(t, "Renamed", s.AITitle)
}
