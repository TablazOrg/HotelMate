# HotelMate recovery-set and restore runbook

HotelMate treats PostgreSQL plus the approved private-upload scope as one `hotelmate.recovery-set/v1` at a shared UTC timestamp. The manifest records release/schema evidence, file sizes, SHA-256 digests, off-host state, and restic snapshot identity. A dump without its manifest is not an approved recovery set.

## Create, verify, and retain

Use the protected environment whose `DATABASE_URL`, backup directory, restic repository/password, and retention are already approved:

```bash
hotelmate --config /etc/hotelmate/production.env backup create --yes --json
hotelmate --config /etc/hotelmate/production.env backup list
hotelmate --config /etc/hotelmate/production.env \
  --manifest /srv/hotelmate-private/backups/hotelmate-<timestamp>.json backup verify
```

Creation uses restrictive file/directory modes. It rejects an empty/truncated dump, recomputes SHA-256, and validates the PostgreSQL custom-format catalog. Manifest parsing permits exactly one JSON document and base-name-only artifact references, preventing traversal outside the catalog. When the upload directory contains documents, they are archived without symlinks and the restore rejects archive traversal. Restic encrypts before off-host transfer, must return a snapshot ID, and prunes only the matching HotelMate/environment snapshots to the configured daily/weekly policy; production deploy preflight requires that destination.

The systemd timer runs daily. Alert when the last successful backup is older than 26 hours, verification/transfer/retention fails, or disk capacity falls below policy. Ensure at least one repository is outside the runtime host/account failure domain.

## Isolated restore drill

Never point a rehearsal at production. Create an isolated PostgreSQL database, upload path, application environment, and domain. Set `HOTELMATE_ISOLATED_RECOVERY_DRILL=true` only in that protected configuration. The automated command selects the latest set at or before the requested point:

```bash
hotelmate --config /etc/hotelmate/recovery-drill.env \
  --requested-recovery-point 2026-08-23T00:00:00Z \
  --operator scheduled-drill backup drill --yes --json
```

It executes and records the following sequence:

1. Record drill start, source recovery timestamp, requested recovery point, release, operator, and expected RPO/RTO.
2. Fetch the restic snapshot into a mode-restricted staging directory when the local catalog copy is unavailable.
3. Run checksum/catalog verification for the selected manifest.
4. Run the guarded restore into the configured isolated target.
5. The upload restore preserves an existing target as `.pre-restore-<timestamp>` before activating restored files; retain or remove that copy under the approved sensitive-data policy.
6. Apply and verify the migration ledger.
7. Run public/authenticated smoke and the native stateful acceptance suite, including tenant isolation and private-document access, against the already isolated matching application release.
8. Record finish time, actual RPO/RTO, checks, gaps, and evidence under `HOTELMATE_EVIDENCE_DIR`; update the restore-drill metric and securely expire drill data.

The optional systemd timer runs monthly only when `hotelmate_recovery_drill_enabled` is approved and a Vault-protected `hotelmate_recovery_drill_env` is supplied. A production `APP_ENV` target is rejected before artifact access.

If logical dumps cannot meet the approved RPO, do not weaken the target: add PostgreSQL physical backup/WAL archiving and extend the manifest/drill contract.

## Production incident restore

Restoration cleans/replaces matching database objects and therefore always requires `--yes`.

1. Declare the incident and maintenance window; freeze application writes.
2. Preserve database/host forensic state. Take a final recovery set if corruption has not made it unsafe.
3. Verify target environment, selected manifest, checksums/catalog, release/schema, and restic snapshot with two-person review when policy requires it.
4. Stop API/web, run the guarded restore command, apply reviewed forward migrations, and deploy the matching or approved compatible image manifest.
5. Run readiness, public/authenticated smoke, tenant-isolation acceptance, private-document sampling under authorization, audit/report checks, and monitoring/alert checks.
6. Resume traffic only with incident-commander approval. Record actual RPO/RTO and schedule follow-up review.

Credential loss, complete host loss, certificate failure, accidental deletion, and compromised release procedures are in [OPERATIONS_RUNBOOK.md](OPERATIONS_RUNBOOK.md).
