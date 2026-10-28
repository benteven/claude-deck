package gh

import "testing"

func TestRollup(t *testing.T) {
	done := func(c string) check { return check{Status: "COMPLETED", Conclusion: c} }
	running := check{Status: "IN_PROGRESS"}
	cases := []struct {
		checks []check
		want   string
	}{
		{nil, ""},
		{[]check{done("SUCCESS"), done("SKIPPED"), {State: "SUCCESS"}}, "pass"},
		{[]check{done("SUCCESS"), running}, "pending"},
		{[]check{running, done("FAILURE")}, "fail"},
		{[]check{done("SUCCESS"), {State: "ERROR"}}, "fail"},
	}
	for _, c := range cases {
		if got := rollup(c.checks); got != c.want {
			t.Errorf("rollup(%v) = %q, want %q", c.checks, got, c.want)
		}
	}
}
