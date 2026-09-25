# dbtote

A cross-platform CLI tool for backing up and restoring databases, with compression, encryption at rest, and pluggable storage backends.

Full design: [`docs/SPEC.md`](docs/SPEC.md).

## Status

Phase 4: MySQL and PostgreSQL full backup/restore, local and S3 storage, gzip compression, age encryption, OS-keyring credentials, YAML config file with targets, `test-connection`, `list` (backed by a local SQLite metadata index), retention policy enforcement, Slack success/failure notifications, and CI/CD (gated releases, Docker image, keyless artifact signing, PR coverage comments).

## Install

Download a prebuilt binary from the [latest release](https://github.com/nweber23/dbtote/releases/latest), or pull the Docker image:

```bash
docker pull ghcr.io/nweber23/dbtote:latest
```

Or build from source:

```bash
git clone https://github.com/nweber23/dbtote.git
cd dbtote
go build -o bin/dbtote ./cmd/dbtote
```

Requires the `mysqldump`/`mysql` client binaries on `PATH` for MySQL targets, and `pg_dump`/`pg_restore` for PostgreSQL targets.

## Quick start

1. Store the database password once:

   ```bash
   dbtote secret set --target prod-mysql
   ```

2. Run a backup:

   ```bash
   dbtote backup --target prod-mysql --host db.internal --user backup_svc --database app_production --output /var/backups/dbtote
   ```

3. Encrypt backups at rest with [age](https://github.com/FiloSottile/age):

   ```bash
   age-keygen -o identity.txt   # prints the matching public key
   dbtote backup --target prod-mysql --host db.internal --user backup_svc --database app_production \
     --output /var/backups/dbtote --encrypt --recipient age1...
   ```

4. Restore:

   ```bash
   dbtote restore --target prod-mysql --from /var/backups/dbtote/prod-mysql_full_....sql.gz.age \
     --host db.internal --user backup_svc --database app_production --decrypt-key identity.txt --yes
   ```

## Config file

Instead of passing `--host`/`--user`/`--database`/etc. on every command, define targets once in a YAML config file. CLI flags always override the config for that run.

1. Write a starter config:

   ```bash
   dbtote config init
   ```

   Prompts for a local storage path and writes `$XDG_CONFIG_HOME/dbtote/config.yaml` (or `--config <path>`).

2. Add a target under `targets:`, e.g.:

   ```yaml
   targets:
     prod-mysql:
       engine: mysql
       host: db1.internal
       port: 3306
       user: backup_svc
       database: app_production
       storage: local-main
   ```

3. Validate the config — checks storage references, age recipient keys, cron expressions, and that every target's credential resolves (`password_env` or OS keyring), without connecting to any database:

   ```bash
   dbtote config validate
   ```

4. Check connectivity and credentials for a specific target:

   ```bash
   dbtote test-connection --target prod-mysql
   ```

5. List known backups:

   ```bash
   dbtote list
   dbtote list --target prod-mysql --since 24h
   dbtote list --json
   ```

With a config file in place, `dbtote backup --target prod-mysql` and `dbtote restore --target prod-mysql --from <file> --yes` resolve engine, host, port, user, database, storage, and encryption recipients from the target's config entry — pass any of `--host`/`--user`/`--database`/etc. explicitly to override just that field for one run.

### PostgreSQL

Postgres is supported the same way MySQL is — set `engine: postgres` on a target. Requires the `pg_dump` and `pg_restore` client binaries on `PATH`.

## S3 storage, retention, and notifications

Add an S3 (or S3-compatible) backend under `storage:` — credentials resolve via the standard AWS chain (env vars, shared config, instance/workload identity), never a dbtote-specific setting:

```yaml
storage:
  s3-main:
    type: s3
    bucket: prod-dbtote-backups
    region: us-east-1
    prefix: dbtote/
```

Set a retention policy under `defaults:` (applies to every target) and/or per-target — `keep_last` and `keep_days` are a union, a backup survives if it satisfies either:

```yaml
defaults:
  retention:
    keep_last: 7
    keep_days: 30
```

See what a policy would delete without deleting anything, then enforce it:

```bash
dbtote retention preview --target prod-mysql
dbtote retention apply --target prod-mysql
```

Get a Slack message on backup success and/or failure — `webhook_url_env` names an env var holding the webhook URL, never the URL itself in the config file:

```yaml
notify:
  slack:
    webhook_url_env: SLACK_WEBHOOK_URL
    on: [success, failure]
```

`dbtote backup` sends notifications by default when `notify.slack` is configured; pass `--notify=false` to skip one run without touching the config.

## Releasing

Pushing a `v*` tag triggers `.github/workflows/release.yml`: it gates on the full CI suite, then runs `goreleaser` to cross-compile binaries for linux/darwin/windows (amd64 + arm64, minus windows/arm64), checksum them, and publish a GitHub Release with an auto-generated changelog. Every release artifact is signed keylessly with [`cosign`](https://github.com/sigstore/cosign) (Sigstore's OIDC-based flow — no signing key to manage), and a multi-arch Docker image is published to `ghcr.io/nweber23/dbtote`:

```bash
docker run --rm ghcr.io/nweber23/dbtote:latest backup --target prod-mysql --host db.internal --user backup_svc --database app_production --output /backups
```

Homebrew isn't set up yet — it needs a separate `nweber23/homebrew-dbtote` tap repository and a `HOMEBREW_TAP_GITHUB_TOKEN` secret, neither of which exist yet (link here once they do).

## Development

```bash
make build   # compile ./cmd/dbtote
make test    # unit tests
make test-integration   # requires Docker; spins up real MySQL via testcontainers
make lint    # golangci-lint
```