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

## Development

```bash
make build   # compile ./cmd/dbtote
make test    # unit tests
make test-integration   # requires Docker; spins up real MySQL via testcontainers
make lint    # golangci-lint
```