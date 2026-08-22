# HotelMate backup and restore runbook

This runbook covers the PostgreSQL system of record for the production Compose deployment. Run commands from the repository deployment directory with the intended `.env.production` already reviewed.

## Create and retain a backup

1. Choose an absolute directory on encrypted storage outside the Docker data volume.
2. Run `./scripts/backup.sh /absolute/private/backup/path`.
3. Confirm that both `hotelmate-<UTC timestamp>.dump` and its `.sha256` file exist and are non-empty.
4. Copy them to the approved off-host destination and apply the retention policy.
5. Back up the private uploads volume separately only when the document recovery policy requires it; access and expiry must match the sensitive-document policy.

The script uses PostgreSQL custom format so individual objects can be inspected and the full database can be restored with `pg_restore`. File creation uses a restrictive umask. The storage destination must provide encryption at rest.

## Restore drill or incident recovery

Restoration is destructive to matching objects in the configured target database. Never point a rehearsal at production.

1. Identify the exact target host and database, announce the maintenance window, and take a final backup when the source is still available.
2. Verify the dump checksum using `sha256sum -c <dump>.sha256` or `shasum -a 256 -c <dump>.sha256`.
3. Stop writes with `docker compose --env-file .env.production -f docker-compose.production.yml stop api`.
4. Run `./scripts/restore.sh /absolute/path/hotelmate.dump --confirm`.
5. Re-run migrations with `docker compose --env-file .env.production -f docker-compose.production.yml run --rm --entrypoint /app/migrate api`.
6. Start the API and web services with `docker compose --env-file .env.production -f docker-compose.production.yml up -d`.
7. Run `./scripts/smoke.sh https://<domain>`. For authenticated checks, set `SMOKE_HOTEL_SLUG`, `SMOKE_STAFF_EMAIL`, and `SMOKE_STAFF_PASSWORD` to a dedicated administrator or operations-manager smoke account.
8. Confirm `/readyz`, recent request logs, the reporting dashboard, tenant isolation, and scheduled purge jobs before ending the maintenance window.

Record the backup timestamp, Git commit, migration ledger contents, restore duration, smoke result, and operator. A successful backup without a recurring restore drill is not considered recoverable.
