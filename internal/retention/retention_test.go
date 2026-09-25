package retention

import (
	"testing"
	"time"
)

func backupsAt(days ...int) []Backup {
	now := time.Now()
	var out []Backup
	for _, d := range days {
		out = append(out, Backup{Filename: fmt("b", d), Timestamp: now.Add(-time.Duration(d) * 24 * time.Hour)})
	}
	return out
}

func fmt(prefix string, n int) string {
	return prefix + string(rune('0'+n))
}

func TestApply_KeepsUnionOfKeepLastAndKeepDays(t *testing.T) {
	// 5 backups, one per day for the last 5 days (0..4 days old).
	backups := backupsAt(0, 1, 2, 3, 4)

	pruned := Apply(Policy{KeepLast: 2, KeepDays: 1}, backups)

	// keep_last=2 keeps the two newest (day 0, day 1); keep_days=1 keeps
	// anything younger than 1 day (day 0 only). Union: days 0 and 1 survive;
	// days 2, 3, 4 are pruned.
	if len(pruned) != 3 {
		t.Fatalf("got %d pruned, want 3: %v", len(pruned), pruned)
	}
	prunedSet := map[string]bool{}
	for _, p := range pruned {
		prunedSet[p] = true
	}
	for _, keep := range []string{"b0", "b1"} {
		if prunedSet[keep] {
			t.Errorf("expected %q to be kept, but it was pruned", keep)
		}
	}
	for _, gone := range []string{"b2", "b3", "b4"} {
		if !prunedSet[gone] {
			t.Errorf("expected %q to be pruned", gone)
		}
	}
}

func TestApply_ZeroPolicyPrunesNothing(t *testing.T) {
	backups := backupsAt(0, 100, 200)
	pruned := Apply(Policy{}, backups)
	if len(pruned) != 0 {
		t.Errorf("expected a zero-value policy to prune nothing, got %v", pruned)
	}
}
