# dbtote

A cross-platform CLI tool for backing up and restoring databases, with compression, encryption at rest, and pluggable storage backends.

Full design: [`docs/SPEC.md`](docs/SPEC.md).

## Status

Phase 1: MySQL full backup/restore, local storage, gzip compression, age encryption, OS-keyring credentials.

## Install

```bash
go install github.com/nweber23/dbtote/cmd/dbtote@latest
```

Requires the `mysqldump` and `mysql` client binaries on `PATH`.

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

## Development

```bash
make build   # compile ./cmd/dbtote
make test    # unit tests
make test-integration   # requires Docker; spins up real MySQL via testcontainers
make lint    # golangci-lint
```