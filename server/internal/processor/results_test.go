package processor

import (
	"testing"
	"time"
)

func TestReclassifyStatus(t *testing.T) {
	now := time.Now()
	cases := []struct {
		status    string
		notAfter  time.Time
		threshold int
		want      string
	}{
		{"ok", now.Add(10 * 24 * time.Hour), 15, "expiring"},
		{"ok", now.Add(20 * 24 * time.Hour), 15, "ok"},
		{"expired", now.Add(5 * 24 * time.Hour), 15, "expired"},
		{"mismatch", now.Add(5 * 24 * time.Hour), 15, "mismatch"},
		{"unreachable", now.Add(5 * 24 * time.Hour), 15, "unreachable"},
	}
	for _, c := range cases {
		got := ReclassifyStatus(c.status, c.notAfter, c.threshold)
		if got != c.want {
			t.Errorf("ReclassifyStatus(%q, +%v days, %d) = %q, want %q",
				c.status, c.notAfter.Sub(now).Hours()/24, c.threshold, got, c.want)
		}
	}
}
