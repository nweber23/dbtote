package backupjob

import (
	"strings"
	"time"
)

func ParseFilename(name string) (target, backupType string, ts time.Time, ok bool) {
	parts := strings.SplitN(name, "_", 3)
	if len(parts) != 3 {
		return "", "", time.Time{}, false
	}
	target, backupType = parts[0], parts[1]

	rest := parts[2]
	// The timestamp is exactly "2006-01-02T15-04-05Z" (20 chars); whatever
	// follows is the extension chain.
	const stampLen = len("2006-01-02T15-04-05Z")
	if len(rest) < stampLen {
		return "", "", time.Time{}, false
	}
	stamp := rest[:stampLen]
	parsed, err := time.Parse("2006-01-02T15-04-05Z", stamp)
	if err != nil {
		return "", "", time.Time{}, false
	}
	return target, backupType, parsed, true
}
