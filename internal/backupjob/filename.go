package backupjob

import "time"

// BuildFilename produces names like "prod-mysql_full_2026-09-22T02-00-00Z.sql.gz.age"
func BuildFilename(target, backupType string, ts time.Time, extChain string) string {
	stamp := ts.UTC().Format("2006-01-02T15-04-05Z")
	return target + "_" + backupType + "_" + stamp + extChain
}
