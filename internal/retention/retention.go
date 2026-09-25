package retention

import (
	"sort"
	"time"
)

type Policy struct {
	KeepLast int
	KeepDays int
}

type Backup struct {
	Filename  string
	Timestamp time.Time
}

func Apply(policy Policy, backups []Backup) []string {
	if policy.KeepLast <= 0 && policy.KeepDays <= 0 {
		return nil
	}

	sorted := make([]Backup, len(backups))
	copy(sorted, backups)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp.After(sorted[j].Timestamp) })

	keep := map[string]struct{}{}
	if policy.KeepLast > 0 {
		for i := 0; i < policy.KeepLast && i < len(sorted); i++ {
			keep[sorted[i].Filename] = struct{}{}
		}
	}
	if policy.KeepDays > 0 {
		cutoff := time.Now().Add(-time.Duration(policy.KeepDays) * 24 * time.Hour)
		for _, b := range sorted {
			if b.Timestamp.After(cutoff) {
				keep[b.Filename] = struct{}{}
			}
		}
	}

	var prune []string
	for _, b := range sorted {
		if _, ok := keep[b.Filename]; !ok {
			prune = append(prune, b.Filename)
		}
	}
	return prune
}
