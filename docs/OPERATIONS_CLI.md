# HotelMate operations CLI

`hotelmate` is the single operator interface for configuration validation, migrations, recovery sets, retention, deployment, rollback, smoke checks, and release identity. CI uses the same binary that is shipped in the API image and release bundle.

## Configuration and safety contract

Resolution order is flags, process environment, then the optional `--config` file. A config file must be a regular file inaccessible to group and other users (`chmod 600`). Secrets are used but never included in human or JSON results.

The stable process exit codes are:

| Code | Meaning |
| --- | --- |
| `0` | Command and verification succeeded |
| `2` | Invalid command or configuration |
| `3` | Failed precondition, including missing confirmation or tooling |
| `4` | Mutating command failed |
| `5` | Post-command or artifact verification failed |

`--json` writes one `hotelmate.operations/v1` envelope to stdout. Diagnostics go to stderr. Automation should inspect both the exit code and `ok`; it must not parse human prose.

Every mutating command requires `--yes` in every environment: migration up, backup create/restore/drill, retention purges, deploy apply, and deploy rollback. Deploy, migration, restore, and recovery drill share an environment ownership lock. A live owner is never displaced; a verifiably dead same-host owner is archived for audit, and malformed or remote-host locks require operator review. Production deploy creates a verified off-host recovery checkpoint before migrations.

The process handles `SIGINT` and `SIGTERM` through cancellation and releases only a lock whose random owner token still matches. Every scheduled/mutating job writes last-run, last-success, and success-state metrics for the node-exporter textfile collector without erasing a prior successful timestamp on failure.

## Command summary

```text
hotelmate doctor
hotelmate config validate [--environment staging|production]
hotelmate migrate status|up [--dry-run]
hotelmate backup create|list|verify|restore|drill
hotelmate retention purge-documents|purge-messages
hotelmate deploy preflight|apply|status|rollback
hotelmate smoke
hotelmate acceptance
hotelmate version
```

Use `hotelmate --help` for global path overrides. The most common production pattern is:

```bash
hotelmate --config /etc/hotelmate/production.env --release-file /srv/hotelmate/incoming/release.json doctor
hotelmate --config /etc/hotelmate/production.env --release-file /srv/hotelmate/incoming/release.json deploy preflight
hotelmate --config /etc/hotelmate/production.env --release-file /srv/hotelmate/incoming/release.json deploy apply --yes
hotelmate --config /etc/hotelmate/production.env deploy status --json
```

## Recovery sets

`backup create --yes` produces a mode-`600` PostgreSQL custom-format dump, optional private-upload archive, and `hotelmate.recovery-set/v1` manifest with SHA-256 digests, byte lengths, release version, migration ledger, and one recovery timestamp. `backup verify` rejects extra JSON documents, unsafe artifact names, bad hashes, undersized files, missing off-host snapshot identity, and malformed metadata; it recomputes every digest and asks the matching PostgreSQL `pg_restore` to inspect a non-empty catalog.

`HOTELMATE_POSTGRES_DRIVER=direct` uses installed PostgreSQL client tools and a parsed `DATABASE_URL`; the password is supplied through the child environment rather than the process command line. `compose` executes the PostgreSQL 16 tools in the Compose database container and is convenient for development.

When `RESTIC_REPOSITORY` and `RESTIC_PASSWORD` are set, the complete recovery set is encrypted and transferred by restic before the command succeeds. The CLI then enforces `RESTIC_KEEP_DAILY` and `RESTIC_KEEP_WEEKLY`. Production deploy preflight rejects a missing off-host repository.

`backup drill --yes` is permitted only for a non-production target with `HOTELMATE_ISOLATED_RECOVERY_DRILL=true`. It selects the newest recovery set at or before `--requested-recovery-point` (or uses `--manifest`), verifies/restores it, applies migrations, runs public and authenticated smoke plus the native stateful acceptance suite, measures RPO/RTO, records `hotelmate.recovery-drill/v1`, and updates the restore-drill metric. Set `--operator`, `HOTELMATE_DRILL_MAX_RPO`, and `HOTELMATE_DRILL_MAX_RTO` to make the evidence enforce approved objectives.

## Immutable release manifest

Staging and production accept exactly one `hotelmate.release/v1` JSON document. Both image fields must be full `image@sha256:<64-hex-digest>` references, and the migration set must equal the operations binary. The manifest records commit, release version, creation time, migrations, API/web SBOMs, and CI/scan/provenance/signature/attestation evidence. CI signs each image and its SPDX attestation with keyless cosign, verifies them, and promotes the same manifest to both environments. Host-side preflight verifies the signature and attestation again against the configured workflow identity and GitHub OIDC issuer before pulling.

Application rollback changes only the API/web image references and reruns smoke checks. It never attempts an automatic destructive database rollback. The exact release, previous release, checkpoint, result, and timestamps are retained under `HOTELMATE_EVIDENCE_DIR`.

Staging/production configuration validation requires all three dedicated authenticated-smoke values: `SMOKE_HOTEL_SLUG`, `SMOKE_STAFF_EMAIL`, and `SMOKE_STAFF_PASSWORD`. The `acceptance` command additionally requires `ACCEPTANCE_ONBOARDING_TOKEN`, creates isolated test tenants, and therefore runs in staging/drill environments rather than production.
