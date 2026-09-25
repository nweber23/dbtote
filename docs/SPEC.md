# dbtote — Full Project Specification

A cross-platform CLI tool for backing up and restoring databases (MySQL, PostgreSQL, MongoDB, SQLite, and more), with compression, encryption at rest, local/cloud storage, scheduling, logging, and notifications. Built in Go, released via a full CI/CD pipeline.

> Naming note: `dbtote` was checked against GitHub/pkg.go.dev/PyPI at planning time with no direct collisions. Re-verify the exact GitHub org/repo name and module path before publishing.

This document supersedes the earlier skeleton plan. It is the spec to build against.

---

## 1. Core Architecture

Everything is an interface, following the same driver-registration pattern Go's own `database/sql` uses. This is what makes "support every DBMS" tractable without a rewrite every time a new engine is added.

```go
// Connector: talks to a specific DBMS
type Connector interface {
    Connect(ctx context.Context, cfg ConnectionConfig) error
    Ping(ctx context.Context) error
    Close() error
}

// Backuper: knows how to dump a specific DBMS
type Backuper interface {
    Backup(ctx context.Context, opts BackupOptions) (BackupResult, error)
    SupportsIncremental() bool
    SupportsDifferential() bool
    // IncrementalBasis describes what an incremental backup is anchored to
    // for this engine (e.g. "wal_lsn", "binlog_position", "oplog_timestamp").
    IncrementalBasis() IncrementalBasisKind
}

// Restorer: knows how to restore a specific DBMS
type Restorer interface {
    Restore(ctx context.Context, opts RestoreOptions) error
    SupportsSelective() bool // table/collection-level restore
}

// StorageBackend: where the backup file/stream ends up
type StorageBackend interface {
    Store(ctx context.Context, name string, r io.Reader) error
    Retrieve(ctx context.Context, name string) (io.ReadCloser, error)
    List(ctx context.Context, prefix string) ([]BackupMeta, error)
    Delete(ctx context.Context, name string) error
}

// SecretStore: where credentials live
type SecretStore interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    Delete(ctx context.Context, key string) error
}
```

Each DBMS implements `Connector` + `Backuper` + `Restorer` in its own package and self-registers via `init()` into `internal/driver/registry.go`. Each storage target implements `StorageBackend`. The CLI/orchestration layer never knows about `mysqldump` vs `pg_dump` directly.

---

## 2. Package Layout

```
dbtote/
├── cmd/
│   └── dbtote/
│       └── main.go
├── internal/
│   ├── cli/                        # cobra command definitions
│   │   ├── root.go
│   │   ├── backup.go
│   │   ├── restore.go
│   │   ├── schedule.go
│   │   ├── daemon.go
│   │   ├── config.go
│   │   ├── secret.go                # `dbtote secret set/get/rm`
│   │   ├── test.go                  # `dbtote test-connection`
│   │   └── version.go
│   ├── config/                      # config file parsing/validation (viper)
│   │   ├── config.go
│   │   └── schema.go
│   ├── secrets/
│   │   ├── keyring.go                # OS keyring via go-keyring
│   │   └── types.go
│   ├── crypto/
│   │   └── age.go                    # age encrypt/decrypt streaming wrappers
│   ├── driver/
│   │   ├── registry.go
│   │   └── types.go
│   ├── db/
│   │   ├── mysql/
│   │   ├── postgres/
│   │   ├── mongodb/
│   │   ├── sqlite/
│   │   └── ...
│   ├── storage/
│   │   ├── local/
│   │   ├── s3/
│   │   ├── gcs/
│   │   ├── azureblob/
│   │   └── registry.go
│   ├── compress/                     # gzip/zstd wrappers
│   ├── scheduler/
│   │   ├── cron.go
│   │   └── daemon.go
│   ├── notify/
│   │   └── slack/
│   ├── logging/                      # structured logging (slog-based)
│   ├── retention/                    # retention policy enforcement
│   └── backupjob/                    # orchestrates: connect → dump → compress → encrypt → store → log → notify
├── pkg/
├── docs/
│   ├── cli-reference.md
│   └── config-reference.md
├── scripts/
├── testdata/
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── docker-compose.dev.yml            # MySQL/PG/Mongo for local dev + integration tests
├── Dockerfile
├── goreleaser.yml
├── Makefile
├── go.mod
├── go.sum
├── README.md
├── CHANGELOG.md
└── LICENSE
```

---

## 3. Dependencies

| Purpose | Package | Why |
|---|---|---|
| CLI framework | `github.com/spf13/cobra` | Standard for Go CLIs (kubectl/docker/gh) |
| Config | `github.com/spf13/viper` | Pairs with cobra; YAML/env/flags |
| MySQL driver | `github.com/go-sql-driver/mysql` | De facto standard |
| Postgres driver | `github.com/jackc/pgx/v5` | Actively maintained, faster than lib/pq |
| MongoDB driver | `go.mongodb.org/mongo-driver` | Official |
| SQLite driver | `modernc.org/sqlite` | Pure Go, no CGo — simplifies cross-compilation |
| AWS S3 | `github.com/aws/aws-sdk-go-v2` | Official |
| GCS | `cloud.google.com/go/storage` | Official |
| Azure Blob | `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob` | Official |
| Compression | stdlib `compress/gzip`; `github.com/klauspost/compress/zstd` | zstd optional, better ratio/speed |
| Encryption | `filippo.io/age` | Modern, keypair-based, streaming-friendly, widely trusted in Go tooling |
| Secrets/keyring | `github.com/zalando/go-keyring` | Wraps macOS Keychain / Windows Credential Manager / Linux Secret Service |
| Cron scheduling | `github.com/robfig/cron/v3` | Standard cron expression parsing/scheduling |
| Slack notify | plain HTTP POST to webhook | no dependency needed |
| Logging | stdlib `log/slog` | structured logging, no dependency |
| Testing | stdlib `testing` + `github.com/stretchr/testify` + `github.com/testcontainers/testcontainers-go` | assertions, mocks, real DB containers in CI |
| Release tooling | `goreleaser` | cross-compilation, GitHub Releases, Docker images, Homebrew tap |

**Dump strategy:** v1 shells out to native tools (`mysqldump`, `pg_dump`, `mongodump`) — simplest path to a working product. A later milestone replaces MySQL/Postgres dumping with pure-Go implementations using the driver directly (streaming SQL/COPY output ourselves), removing the external binary dependency and doubling as a serious Go learning exercise. This is called out explicitly in the roadmap (Phase 9) rather than blocking v1.

---

## 4. CLI Command Surface

Top-level:

```
dbtote [global flags] <command> [command flags]
```

Global flags (available on every command):

```
--config <path>       Path to config file (default: $XDG_CONFIG_HOME/dbtote/config.yaml)
--log-level <level>   debug|info|warn|error (default: info)
--log-format <fmt>    text|json (default: text; json recommended for daemon/CI use)
--no-color            Disable colored output
-v, --version         Print version and exit
-h, --help            Help for any command
```

### `dbtote backup`

Runs a single backup job.

```
dbtote backup --target <name> [flags]
dbtote backup --target <name> --type full|incremental|differential
```

Flags:
```
--target <name>        Name of a database target defined in config (required unless --host etc. given inline)
--type <kind>           full (default) | incremental | differential
--host, --port, --user, --database   Inline connection override (bypasses config target)
--password-env <VAR>    Read password from env var instead of keyring (explicit escape hatch)
--compress <alg>        none|gzip|zstd (default: gzip)
--encrypt                Encrypt output with age (default: true if a recipient key is configured)
--recipient <age-key>    age public key to encrypt to (overrides config)
--storage <name>         Storage backend name from config (default: config default)
--output <path>          For local storage, explicit output path/prefix
--dry-run                 Validate connection + plan, do not execute
--notify / --no-notify    Override notification config for this run
```

Exit codes: `0` success, `1` backup failed, `2` config/validation error, `3` connection failure.

### `dbtote restore`

```
dbtote restore --target <name> --from <backup-id|path> [flags]
```

Flags:
```
--target <name>         Target to restore into
--from <id|path>         Backup identifier (from `dbtote list`) or explicit file/URI
--selective <items>       Comma-separated table/collection names (only if SupportsSelective() for the engine)
--point-in-time <ts>      For incremental chains, restore up to this timestamp (RFC3339)
--decrypt-key <path>      age private key file (default: from keyring)
--dry-run                  Show restore plan without executing
--confirm                  Required for destructive restores unless --yes
--yes                      Skip confirmation prompt
```

Exit codes mirror `backup`.

### `dbtote list`

Lists known backups (queries configured storage backends + local metadata index).

```
dbtote list [--target <name>] [--storage <name>] [--since <duration>] [--json]
```

### `dbtote test-connection`

Validates credentials/connectivity for a configured target without performing a backup.

```
dbtote test-connection --target <name>
```

### `dbtote secret`

Manages credentials in the OS keyring.

```
dbtote secret set --target <name>       # prompts for password, stores in keyring
dbtote secret get --target <name>        # prints (for debugging; requires --yes-i-know)
dbtote secret rm  --target <name>
dbtote secret list
```

### `dbtote schedule`

Manages scheduled jobs when using external cron/systemd (i.e. non-daemon mode) — this subcommand mainly generates the crontab line / systemd timer unit for the user, it does not itself run in the background.

```
dbtote schedule add --target <name> --cron "0 2 * * *" --type full
dbtote schedule add --target <name> --cron "0 * * * *" --type incremental
dbtote schedule list
dbtote schedule rm <id>
dbtote schedule print-crontab           # emits crontab-compatible lines
dbtote schedule print-systemd-timer <id> # emits a systemd .timer + .service unit pair
```

### `dbtote daemon`

Runs dbtote as a long-lived process with its own in-process scheduler (optional layer on top of the plain CLI — same `schedule` definitions drive it).

```
dbtote daemon run [--pidfile <path>] [--foreground]
```

In daemon mode, `dbtote schedule add` entries are read from config/state and executed in-process via `robfig/cron` instead of relying on external cron. Same `backupjob` orchestration code path either way — daemon mode is just an in-process trigger, not a different backup engine.

### `dbtote config`

```
dbtote config validate [--config <path>]
dbtote config init                        # interactive wizard, writes a starter config.yaml
```

### `dbtote retention`

```
dbtote retention apply --target <name>    # enforce retention policy now (normally automatic post-backup)
dbtote retention preview --target <name>  # show what would be deleted
```

### `dbtote version`

Prints version, commit hash, build date, Go version (populated by goreleaser via `-ldflags`).

---

## 5. Config File Schema

YAML, loaded via viper, with env var overrides using `DBTOTE_` prefix and `_` as the nesting separator (e.g. `DBTOTE_TARGETS_PROD_HOST`).

```yaml
# ~/.config/dbtote/config.yaml
version: 1

defaults:
  compress: gzip            # none | gzip | zstd
  encrypt: true
  storage: local-main       # default storage backend name
  retention:
    keep_last: 7
    keep_days: 30            # both may apply; union kept

encryption:
  recipients:
    - age1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqgpqyqs  # age public key(s)
  identity_file: ~/.config/dbtote/age-identity.txt   # private key, used for restore (or pulled from keyring if omitted)

storage:
  local-main:
    type: local
    path: /var/backups/dbtote
  s3-archive:
    type: s3
    bucket: my-backups-bucket
    region: eu-central-1
    prefix: dbtote/
    # credentials resolved via standard AWS credential chain (env/instance profile/shared config)
  gcs-archive:
    type: gcs
    bucket: my-gcs-bucket
    prefix: dbtote/
  azure-archive:
    type: azureblob
    account: mystorageaccount
    container: dbtote-backups

notify:
  slack:
    webhook_url_env: SLACK_WEBHOOK_URL    # never store the URL in plaintext config
    on: [success, failure]                 # or just [failure]

targets:
  prod-mysql:
    engine: mysql
    host: db1.internal
    port: 3306
    user: backup_svc
    database: app_production
    # password: NOT stored here — resolved via `dbtote secret` / OS keyring
    storage: s3-archive
    incremental:
      enabled: true
      basis: binlog             # mysql: binlog position tracking
      binlog_dir: /var/lib/mysql
    retention:
      keep_last: 14

  prod-postgres:
    engine: postgres
    host: db2.internal
    port: 5432
    user: backup_svc
    database: app_production
    sslmode: require
    storage: s3-archive
    incremental:
      enabled: true
      basis: wal                 # postgres: WAL archiving / pg_basebackup + WAL replay

  prod-mongo:
    engine: mongodb
    uri_env: MONGO_URI            # full URI incl. auth pulled from env, not stored in config
    database: app_production
    storage: gcs-archive
    incremental:
      enabled: true
      basis: oplog

  local-sqlite:
    engine: sqlite
    path: /opt/app/data/app.db
    storage: local-main
    incremental:
      enabled: false              # SQLite: no native incremental concept; differential via file-diff possible later

schedules:
  - id: mysql-nightly-full
    target: prod-mysql
    cron: "0 2 * * *"
    type: full
  - id: mysql-hourly-incremental
    target: prod-mysql
    cron: "0 * * * *"
    type: incremental
```

`dbtote config validate` checks: schema correctness, that every `storage:` reference used by a target/default exists, that encryption recipients parse as valid age keys, that cron expressions parse, and that credentials are resolvable (keyring entry exists or env var is set) — without actually connecting.

---

## 6. Credentials & Secrets Design

- **Primary path**: `dbtote secret set --target <name>` prompts interactively (no echo) and stores the password in the OS keyring under a namespaced key (`dbtote:<target-name>`), via `go-keyring`.
- **Escape hatches** (explicitly less secure, documented as such): `--password-env <VAR>` flag, or a target's `uri_env`/`password_env` config field for CI/container environments where no OS keyring is available (e.g. inside a Docker container in CI — keyring backends generally don't work there).
- **Resolution order** at runtime: explicit CLI flag → env var named in config → OS keyring entry → interactive prompt if running in a TTY and nothing else resolved → hard error.
- Passwords are **never** written to the YAML config file, never logged (the logging layer redacts known secret field names and any value matching common connection-string patterns), and never included in error messages sent to Slack.
- age **identity** (private key) for decryption follows the same resolution order: `--decrypt-key` flag → `identity_file` path in config → keyring entry `dbtote:age-identity` → prompt.

---

## 7. Encryption at Rest (age)

- Backups are encrypted **after** compression, so the pipeline is: dump → compress → encrypt → store. Encrypting before compressing would defeat compression (encrypted data doesn't compress).
- age supports multiple recipients — config's `encryption.recipients` list lets you encrypt one backup to several public keys (e.g. one for automated restore, one held offline for disaster recovery).
- Output file naming reflects the pipeline stages applied, e.g.:
  ```
  prod-mysql_full_2026-09-22T02-00-00Z.sql.gz.age
  prod-postgres_incremental_2026-09-22T03-00-00Z.wal.zst.age
  ```
- `dbtote restore` reverses the pipeline automatically based on the file extension chain; `--decrypt-key` supplies the identity needed for the `.age` layer.
- Encryption can be disabled per-run (`--encrypt=false`) or per-target in config for cases like purely local, already-encrypted-disk backups — but it is **on by default**, matching the requirement to build it in from the start rather than bolt it on.

---

## 8. Per-DBMS Backup Semantics

| Engine | Full backup | Incremental basis | Differential | Selective restore |
|---|---|---|---|---|
| **MySQL** | `mysqldump` (v1) → pure-Go dump (v2) | Binlog position (`SHOW MASTER STATUS`); incremental = binlog events since last full/incremental | Binlog events since last **full** only (not chained) | Yes — restore single table from full dump by filtering the SQL stream, or replay filtered binlog events |
| **PostgreSQL** | `pg_dump` (custom format, v1) → pure-Go COPY-based dump (v2) | WAL archiving (`pg_basebackup` + continuous WAL) | WAL since last full | Yes — `pg_restore -t <table>` against custom-format dumps |
| **MongoDB** | `mongodump --archive` | Oplog timestamp; incremental = oplog entries since last checkpoint | Oplog entries since last full only | Yes — `mongorestore --nsInclude` for specific collections |
| **SQLite** | File copy (via SQLite's own online backup API, safe under concurrent writes) | **Not natively supported** — no binlog/WAL-equivalent exposed for incremental capture in the general case; documented as full-only in v1, differential via whole-file diff considered as a stretch goal | Same caveat as incremental | N/A — single-file database, selective restore doesn't apply the same way; could extract a table via `ATTACH` + `INSERT SELECT` as a future feature |

Design intent: `IncrementalBasisKind` is a real enum (`BasisNone`, `BasisBinlog`, `BasisWAL`, `BasisOplog`) defined now in `internal/driver/types.go`, so every driver package states its capability honestly even where the underlying incremental logic isn't implemented until a later phase. `Backuper.SupportsIncremental()` returns `false` for SQLite from day one rather than silently accepting a flag it can't honor.

---

## 9. Backup Orchestration (`internal/backupjob`)

Single job pipeline, used identically whether triggered by `dbtote backup`, `dbtote daemon`, or external cron:

1. **Resolve config** — merge target config + CLI overrides + secret resolution
2. **Test connection** — fail fast with exit code `3` if unreachable
3. **Acquire lock** — a per-target file lock (e.g. `flock` on a lockfile in the state dir) prevents two overlapping backups of the same target
4. **Dump** — stream DBMS-specific dump output through the pipeline rather than buffering fully in memory where possible (important for "handle large databases efficiently")
5. **Compress** — streaming, e.g. `io.Pipe` between dump and compressor
6. **Encrypt** — streaming age writer wraps the compressed stream
7. **Store** — upload/write via the selected `StorageBackend`; local writes go directly, cloud writes are streamed via multipart upload where the SDK supports it (S3 multipart, etc.) to avoid needing the whole file in memory or on local disk first
8. **Record metadata** — write a `BackupMeta` record (target, type, timestamp, size, checksum, storage location, incremental-basis pointer if applicable) to a local SQLite-based state index, so `dbtote list` and incremental-chain resolution don't require listing every cloud object every time
9. **Apply retention** — prune old backups per policy (skipped if `--dry-run`)
10. **Log** — structured log entry with start time, end time, duration, status, bytes transferred, any error
11. **Notify** — Slack webhook POST if configured for this outcome (`success`/`failure`)

**Retry policy**: only steps 7 (storage upload) and network-dependent parts of step 4 (for remote DBs) are retried, with exponential backoff (3 attempts, base 2s), since these are the failure modes most likely to be transient. A failure at any step aborts the job, cleans up partial output (temp files / incomplete uploads), logs the failure with full context, and still fires the `failure` notification — a failed backup must never look like a silent no-op.

**Concurrency**: multiple *different* targets can back up in parallel (goroutine per target, bounded by a `--max-parallel` flag, default runtime NumCPU-based); the same target is always serialized via the lock in step 3.

---

## 10. Storage Backend Contract Details

Beyond the basic interface (`Store`/`Retrieve`/`List`/`Delete`):

- **Local**: writes to `path/<target>/<filename>`, uses `os.Rename` from a temp file for atomicity (never leaves a partially-written file at the final name).
- **S3 / GCS / Azure**: all three wrap their SDK's streaming upload APIs so `Store` accepts an `io.Reader` and never requires knowing the total size up front (important since compressed+encrypted output size isn't known until the stream ends). Credentials resolved via each SDK's standard credential chain (env vars, shared config files, instance/workload identity) — dbtote does not reinvent cloud credential storage.
- **List** returns `BackupMeta` (name, size, timestamp, target, type) reconstructed either from object metadata/tags (cloud) or from the local state index (local), kept consistent so `dbtote list` output looks identical regardless of backend.

For local dev/testing (per your input), cloud backends are tested against **local substitutes**: Adobe S3Mock (S3-compatible) for the S3 backend — MinIO's community images were pulled from Docker Hub and Quay.io in September 2026, and LocalStack dropped its free tier in March 2026, so S3Mock is the zero-auth alternative — and the official GCS/Azure emulators (`fake-gcs-server`, Azurite) similarly, this gives real integration coverage without needing live cloud credentials or cost.

---

## 11. Logging Schema

Structured via `log/slog`, one JSON object per log line in `--log-format json` mode:

```json
{
  "time": "2026-09-22T02:00:03Z",
  "level": "INFO",
  "msg": "backup completed",
  "target": "prod-mysql",
  "type": "full",
  "job_id": "b7e1...",
  "duration_ms": 48213,
  "bytes_written": 104857600,
  "storage": "s3-archive",
  "status": "success"
}
```

On failure, `status: "failure"`, plus `error` (message, redacted) and `stage` (which pipeline step failed). Log level `debug` additionally logs each pipeline stage transition; `info` logs job start/end only; secrets are redacted by a dedicated `slog.Handler` wrapper that scrubs known sensitive keys before any output is written, so redaction isn't dependent on every call site remembering to do it manually.

---

## 12. Testing Strategy

- **Unit tests**: pure logic — config parsing/validation, retention policy calculation, backup filename generation, incremental-basis state tracking, redaction logic. No real DB or network needed; run on every `go test ./...`.
- **Integration tests** (build-tagged `//go:build integration`, run separately in CI): spin up real MySQL/Postgres/MongoDB via `testcontainers-go`, exercise full backup → restore round-trips, verify data integrity after restore (row counts / checksums match), exercise incremental chains against real binlog/WAL/oplog.
- **Storage backend tests**: against S3Mock/Azurite/fake-gcs-server containers, same pattern.
- **End-to-end smoke test**: one test that runs the actual compiled binary against a full pipeline (real DB container → backup → encrypt → store locally → restore → verify), used as a release gate.
- Target **>75% coverage** on `internal/` excluding thin CLI wiring, enforced via `go test -race -cover` in CI (not a hard gate that blocks merges initially, but tracked and shown on PRs via a coverage comment).

---

## 13. CI/CD Pipeline

### `ci.yml` (every push/PR)
1. `go vet ./...`
2. `golangci-lint run`
3. `go test ./... -race -cover` (unit tests)
4. Build matrix: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64` — compile-only check, catches platform-specific breakage early
5. `go test -tags=integration ./...` against `testcontainers-go`-managed MySQL/Postgres/Mongo containers and Adobe S3Mock/Azurite containers (this job can be slower/separate from the fast unit-test job)

### `release.yml` (on `v*` tag push)
1. Run full CI suite as a gate
2. `goreleaser release` — cross-compiles all platform binaries, generates SHA256 checksums, builds and pushes a multi-arch Docker image, creates the GitHub Release with changelog generated from Conventional Commits, publishes a Homebrew formula to a tap repo
3. Sign release artifacts (e.g. via `cosign` or GPG) — worth adding once the basic pipeline is solid, since backup tools are exactly the kind of software where supply-chain integrity matters

### Versioning
Semantic versioning, strictly tied to Conventional Commits: `fix:` → patch, `feat:` → minor, `BREAKING CHANGE:` footer or `!` → major. `goreleaser` or `git-chglog` derives the changelog and next version automatically from commit history since the last tag.

---

## 15. Phased Roadmap

- **Phase 0** — repo scaffold, cobra skeleton, `--help`/`version` work, CI pipeline running (even with nothing to test yet)
- **Phase 1** — MySQL full backup+restore via shell-out, local storage, gzip, age encryption, structured logging, OS keyring for credentials — this is the first genuinely usable release (`v0.1.0`)
- **Phase 2** — Postgres full backup+restore, config file support, `test-connection`, `config validate`
- **Phase 3** — Full CI/CD hardening: integration test matrix, coverage reporting, artifact signing, Homebrew tap, Docker image
- **Phase 4** — S3 storage backend (tested against Adobe S3Mock), Slack notifications, retry/error handling hardening, retention policy enforcement
- **Phase 5** — SQLite support, MongoDB full backup+restore
- **Phase 6** — `dbtote schedule` (crontab/systemd unit generation) + `dbtote daemon` in-process scheduler
- **Phase 7** — Incremental backups: MySQL binlog-based, Postgres WAL-based, MongoDB oplog-based; point-in-time restore
- **Phase 8** — GCS + Azure Blob storage backends; selective (single table/collection) restore across all supported engines
- **Phase 9** — Pure-Go dump implementations for MySQL/Postgres (removing the `mysqldump`/`pg_dump` binary dependency)
- **Phase 10** — Polish: docs site, man pages, shell completion, `dbtote config init` wizard, real-world load testing against large databases

This is the actual "fully working, full-fledged, everything-done" version of the plan — Phase 1 alone is already a genuinely useful, releasable tool; each subsequent phase adds a coherent, shippable increment rather than leaving half-built scope hanging.