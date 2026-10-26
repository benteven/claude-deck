package update

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"0.5.9", "0.5.10", true},
		{"0.5.10", "0.5.9", false},
		{"0.5.5", "0.5.5", false},
		{"dev", "0.6.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.cur, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.cur, c.latest, got)
		}
	}
}
