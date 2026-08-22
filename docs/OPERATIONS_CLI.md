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

Mutating staging and production commands require `--yes`. Restore always requires `--yes`. Environment locks prevent overlapping deploy/rollback operations, and production deploy creates a verified off-host recovery checkpoint before migrations.

## Command summary

```text
hotelmate doctor
hotelmate config validate [--environment staging|production]
hotelmate migrate status|up [--dry-run]
hotelmate backup create|list|verify|restore
hotelmate retention purge-documents|purge-messages
hotelmate deploy preflight|apply|status|rollback
hotelmate smoke
hotelmate acceptance
hotelmate version
```

Use `hotelmate --help` for global path overrides. The most common production pattern is:

```bash
hotelmate --config /etc/hotelmate/production.env doctor
hotelmate --config /etc/hotelmate/production.env --release-file /srv/hotelmate/incoming/release.json deploy preflight
hotelmate --config /etc/hotelmate/production.env --release-file /srv/hotelmate/incoming/release.json deploy apply --yes
hotelmate --config /etc/hotelmate/production.env deploy status --json
```

## Recovery sets

`backup create` produces a mode-`600` PostgreSQL custom-format dump, optional private-upload archive, and `hotelmate.recovery-set/v1` manifest with SHA-256 digests, byte lengths, release version, migration ledger, and one recovery timestamp. `backup verify` recomputes every digest and asks the matching PostgreSQL `pg_restore` to inspect a non-empty catalog.

`HOTELMATE_POSTGRES_DRIVER=direct` uses installed PostgreSQL client tools and a parsed `DATABASE_URL`; the password is supplied through the child environment rather than the process command line. `compose` executes the PostgreSQL 16 tools in the Compose database container and is convenient for development.

When `RESTIC_REPOSITORY` and `RESTIC_PASSWORD` are set, the complete recovery set is encrypted and transferred by restic before the command succeeds. The CLI then enforces `RESTIC_KEEP_DAILY` and `RESTIC_KEEP_WEEKLY`. Production deploy preflight rejects a missing off-host repository.

## Immutable release manifest

Staging and production accept `hotelmate.release/v1` only. Both image fields must be `image@sha256:<digest>` references. The manifest records the commit, release version, migrations, SBOM evidence, and build run. CI signs each image and its SPDX attestation with keyless cosign, verifies them, and promotes the same manifest to both environments. Host-side preflight verifies the signature and attestation again against the configured workflow identity and GitHub OIDC issuer before pulling.

Application rollback changes only the API/web image references and reruns smoke checks. It never attempts an automatic destructive database rollback. The exact release, previous release, checkpoint, result, and timestamps are retained under `HOTELMATE_EVIDENCE_DIR`.
