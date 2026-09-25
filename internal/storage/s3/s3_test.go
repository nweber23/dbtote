package s3

import "testing"

func TestKey_JoinsPrefixAndName(t *testing.T) {
	b := &Backend{prefix: "dbtote/"}
	if got := b.key("backup.sql.gz"); got != "dbtote/backup.sql.gz" {
		t.Errorf("key() = %q, want %q", got, "dbtote/backup.sql.gz")
	}
}

func TestKey_NoPrefix(t *testing.T) {
	b := &Backend{prefix: ""}
	if got := b.key("backup.sql.gz"); got != "backup.sql.gz" {
		t.Errorf("key() = %q, want %q", got, "backup.sql.gz")
	}
}
