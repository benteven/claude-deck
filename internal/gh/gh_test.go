package gh

import "testing"

func TestRollup(t *testing.T) {
	done := func(name, c string) check { return check{Name: name, Status: "COMPLETED", Conclusion: c} }
	cases := []struct {
		checks []check
		want   string
	}{
		{nil, ""},
		{[]check{done("a", "SUCCESS"), done("b", "SKIPPED"), {Context: "c", State: "SUCCESS"}}, "pass"},
		{[]check{done("a", "SUCCESS"), {Name: "b", Status: "IN_PROGRESS"}}, "pending"},
		{[]check{{Name: "a", Status: "IN_PROGRESS"}, done("b", "FAILURE")}, "fail"},
		{[]check{done("a", "SUCCESS"), {Context: "c", State: "ERROR"}}, "fail"},
		{[]check{done("a", "SUCCESS"), done("b", "CANCELLED")}, "pass"},
		{[]check{{Name: "a", StartedAt: "1", Status: "COMPLETED", Conclusion: "FAILURE"}, {Name: "a", StartedAt: "2", Status: "COMPLETED", Conclusion: "SUCCESS"}}, "pass"},
	}
	for _, c := range cases {
		if got := rollup(c.checks); got != c.want {
			t.Errorf("rollup(%v) = %q, want %q", c.checks, got, c.want)
		}
	}
}
