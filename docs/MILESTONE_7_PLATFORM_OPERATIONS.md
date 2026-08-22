# M7 — Platform operations, CLI, CI/CD, backup, and infrastructure

## Outcome

M7 turns the current single-host release baseline into a reproducible operating platform. An authorized operator must be able to validate configuration, ship an immutable release, observe it, back it up, restore it, and roll it back without assembling ad-hoc commands or exposing secrets.

The target remains intentionally provider-neutral until the discovery decisions below are approved. Kubernetes and multi-region failover are not default requirements; they should be introduced only when availability, traffic, or organizational constraints justify their cost.

## Implementation status — platform foundation delivered, external rollout pending

As of 2026-08-22, the repository contains the M7 provider-neutral foundation:

- A tested Go `hotelmate` CLI with protected config-file precedence, secret redaction, `hotelmate.operations/v1` JSON, stable exit codes, migration status/dry-run, recovery-set catalog/verification/restore, guarded retention, deploy preflight/apply/status/rollback, locks, evidence, automatic application rollback, smoke/acceptance, and build identity.
- Digest-only staging/production Compose, a build-once GitHub release pipeline, pinned Actions, dependency/secret/license/IaC and container gates, provenance, SPDX SBOMs, keyless signatures/attestations, staging acceptance, and protected-environment production promotion of the same digest.
- Coordinated PostgreSQL/private-upload `hotelmate.recovery-set/v1` manifests, SHA-256 and PostgreSQL catalog verification, optional encrypted restic off-host transfer/retention, systemd schedules, freshness metrics, and an isolated-restore procedure.
- An Ansible Ubuntu host baseline for non-root/key-only access, firewalling, automatic security patches, Docker/PostgreSQL/restic tooling, protected configuration, job timers, and rebuildable host setup.
- Prometheus application metrics, PostgreSQL/host/container/external probes, Loki/Alloy central logs, Grafana provisioning, and alert rules for availability, errors, latency, TLS, storage, restarts, log delivery, and backup/privacy-job freshness.

Local Go/frontend checks, Compose validation, CLI smoke, a real PostgreSQL dump/catalog verification, the M0–M6 product acceptance suite, and the M7 operational checks are executable in this workspace. M7 remains **in progress**, not complete: [ADR-0007](adr/0007-platform-operations-decisions.md) is unapproved, no external staging/production host or GitHub environment credentials are present, the restic destination and paging receiver are unconfigured, and no public DNS/TLS, promotion, off-host restore drill, alert test, or measured RPO/RTO evidence can yet exist.

### Local verification evidence

The 2026-08-22 development exercise built `hotelmate-api:dev` and `hotelmate-web:dev`, passed deploy preflight, applied the release through the CLI, and recorded successful smoke evidence. The complete stateful acceptance suite passed. A recovery set produced an 89,955-byte PostgreSQL custom dump with 160 catalog entries; it restored into the separately named `hotelmate_m7_restore_20260822` database, reported all six migrations applied, and was removed after the drill. The database restore operation took one second locally. A rollback exercise changed the running API from image ID `45b367…` to the distinct baseline `e25ec5…`, passed smoke, and redeployed `45b367…` with another successful evidence record. The final dependency-upgraded local deployment runs image ID `03ce427…` and passed the same acceptance and metrics checks.

These values prove the local command paths and guards; they are not production RPO/RTO, public availability, off-host durability, or provider rebuild evidence.

## Current-state discovery

| Area | Existing baseline | Gap to close in M7 |
| --- | --- | --- |
| Operations interface | Make targets, shell scripts, and separate Go commands for migration, demo seed, and retention purges | No cohesive CLI, machine-readable contract, remote-target abstraction, deployment preflight, or rollback command |
| CI | GitHub Actions runs migrations twice, Go tests/vet, frontend build, Compose build, smoke checks, and the M0–M6 acceptance suite | No artifact publication, dependency/security scan gate, SBOM, provenance/signing, environment promotion, deployment locking, or CD |
| Runtime | Provider-neutral production Compose with PostgreSQL, one Go API replica, Nginx TLS, private volumes, and health checks | No selected provider/region, provisioned host, infrastructure as code, automated TLS lifecycle, secrets backend, staging environment, or capacity plan |
| Database protection | Guarded `pg_dump`/`pg_restore`, checksum creation, and a manual restore runbook | No scheduler, encrypted off-host transfer, retention enforcement, backup catalog, automated checksum verification, success/failure alert, or measured restore drill |
| Private documents | Isolated `uploads-production` volume with retention purge | No approved recovery policy or automated encrypted backup/restore path coordinated with database recovery |
| Observability | Structured request logs, request IDs, health/readiness endpoints, reports, and audit records | No central log retention, metrics collection, dashboards, alert routing, external uptime check, certificate/disk monitoring, or defined SLOs |
| Security operations | Production secret validation, TLS/HSTS, security headers, rate limits, non-public PostgreSQL, and tenant/RBAC tests | No host-hardening automation, patch policy, registry/image scanning, dependency and secret scanning, artifact signing, access review, or incident runbook |
| Delivery governance | Migration ledger, deployment and backup documentation, smoke and acceptance scripts | No environment protection rules, change approval, release manifest, deployment evidence, rollback rehearsal, or ownership/on-call model |

## Discovery decisions required

These are product/operations decisions, not values the implementation should guess:

1. Hosting provider, region, data-residency constraint, monthly budget, and account owner.
2. Production and staging domains, DNS provider, certificate owner, and whether IPv6 is required.
3. Availability target, expected hotel/guest/request volume, maintenance-window policy, and scaling trigger.
4. Recovery point objective (RPO), recovery time objective (RTO), backup retention, legal holds, and whether private identity documents are recoverable or intentionally ephemeral.
5. Required environments and promotion policy: pull request, staging, production, emergency hotfix, and approval owners.
6. Container registry, secrets manager, object-storage provider, encryption-key owner, and key-rotation policy.
7. Monitoring/logging provider, data retention, alert destinations, primary responder, and escalation path.
8. SSH and administrator access model, least-privilege roles, MFA requirements, break-glass process, and access-review cadence.

Each decision must be captured in a short architecture decision record under `docs/adr/` before the dependent implementation is considered stable.

The decision record is [ADR-0007](adr/0007-platform-operations-decisions.md). It is intentionally `Proposed` until the eight owner inputs are supplied.

## Work packages

### M7.1 — Operations CLI

Create a Go CLI at `backend/cmd/hotelmate` and move shared operational behavior into importable packages rather than invoking shell commands from Go. The initial command contract is:

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

Requirements:

- Flags override environment variables, which override an optional mode-restricted config file; resolved secrets are never printed.
- Mutating or destructive commands identify the target environment and require an explicit confirmation or CI-safe confirmation flag.
- `--json` emits a versioned schema, stdout contains results, stderr contains diagnostics, and exit codes distinguish invalid configuration, failed preconditions, command failure, and verification failure.
- `doctor` checks required tools, target connectivity, DNS, TLS, Compose/registry access, disk headroom, database readiness, and backup destination access without changing state.
- CLI unit tests cover parsing and redaction; integration tests cover migrations, backup verification, restore guards, smoke checks, and failure exit codes.
- Existing Make targets remain thin developer aliases that call the CLI, preventing two independent operational implementations.

### M7.2 — CI and software supply chain

Extend pull-request and main-branch CI with:

- Formatting, vet/static analysis, unit and PostgreSQL integration tests, frontend type/build checks, migration rehearsal, Compose smoke, and M0–M6 acceptance.
- Dependency, secret, license-policy, and container vulnerability scanning with an explicitly approved severity policy.
- Reproducible API/web OCI images tagged by Git commit and release version, referenced by digest after build.
- OCI image publication to the selected registry, an SBOM for each image, build provenance, signing, and signature verification before deployment.
- Pinned action/tool versions, minimal GitHub token permissions, cache isolation, and branch protection requiring all release gates.
- A release manifest containing Git commit, image digests, schema migration set, SBOM locations, and test/scan evidence.

### M7.3 — Continuous delivery and rollback

Implement a deployment workflow that:

1. Automatically deploys the immutable main-branch image digests to staging.
2. Runs `hotelmate deploy preflight`, migrations, smoke checks, and the stateful acceptance suite against staging.
3. Promotes the exact tested digests to production only after an environment approval.
4. Acquires an environment concurrency lock and prevents overlapping deploy/migration/restore operations.
5. Creates and verifies the policy-required backup checkpoint before a production migration.
6. Waits for readiness, runs public and authenticated smoke checks, records deployment evidence, and alerts on failure.
7. Rolls application images back to the last known-good digest. Database rollback uses an explicitly reviewed forward-fix or restore plan; it is never inferred from an image rollback.

The pipeline must support an audited emergency deployment without bypassing artifact verification, backup policy, or post-deploy smoke checks.

### M7.4 — Backup and disaster recovery

Replace the manual-only database process with a scheduled, monitored backup system:

- Back up PostgreSQL and, if required by the approved document policy, the private upload store as one recovery set with a shared recovery timestamp and release manifest.
- Encrypt before data leaves the host, use a restricted object-storage identity, enable transport encryption, and prohibit public access.
- Generate and verify checksums, inspect PostgreSQL custom-format catalogs, reject empty/truncated artifacts, and record database/schema versions.
- Enforce the approved retention and immutability policy automatically; alert on missed schedules, verification failures, age breaches, and storage-capacity risk.
- Keep at least one copy outside the runtime host/account failure domain when the selected provider permits it.
- Restore into an isolated environment on a scheduled cadence, run migrations and smoke/acceptance checks, measure actual RPO/RTO, and retain drill evidence.
- Document credential loss, host loss, database corruption, accidental deletion, certificate expiry, and compromised-release recovery procedures.

If the approved RPO cannot be met by scheduled logical dumps, add PostgreSQL physical backups and write-ahead-log archiving rather than silently accepting a weaker objective.

### M7.5 — Infrastructure as code and host operations

Codify the selected provider and environment topology, including:

- Network/firewall rules exposing only required SSH, HTTP, and HTTPS paths; PostgreSQL remains private.
- Compute sizing, encrypted persistent volumes, object storage, DNS records, TLS issuance/renewal, registry access, and backup identity.
- Non-root deployment user, key-only SSH, least privilege, time synchronization, unattended security updates or an approved patch window, log rotation, and disk-pressure controls.
- Staging/production separation, secret injection, environment-specific configuration validation, and drift detection.
- Bootstrapping and rebuild-from-zero instructions tested on a replacement host.

Use an IaC tool and configuration-management mechanism selected during discovery. State must be remote, encrypted, access-controlled, backed up, and locked against concurrent writes.

### M7.6 — Observability and operational readiness

Add dashboards and actionable alerts for:

- External HTTPS availability, readiness, latency, HTTP 5xx rate, authentication/security-event anomalies, WebSocket health, and container restarts.
- PostgreSQL connectivity, connections, query latency, storage growth, backup freshness, and restore-drill age.
- Host CPU/memory/disk/inodes, certificate expiry, purge-job failures, deployment failures, and private-volume capacity.
- Release version/image digest and correlation of request IDs across edge and API logs.

Define service-level indicators, objectives, paging thresholds, log/metric retention, ownership, escalation, incident severity, and post-incident review. Alerts must be tested; dashboard existence alone is not operational readiness.

## Delivery sequence

1. Approve discovery decisions, RPO/RTO, environments, provider, and ownership.
2. Build the CLI foundation and configuration contract.
3. Provision staging with infrastructure as code and establish registry/secrets/backup destinations.
4. Harden CI and publish immutable signed artifacts.
5. Automate staging delivery, acceptance, and rollback.
6. Automate backups and pass an isolated restore drill.
7. Provision production, enable monitoring/alerts, and rehearse deployment and rollback.
8. Promote the tested release to production and complete the operational handoff.

## Milestone acceptance gates

M7 is complete only when all of the following evidence exists:

- The CLI help and JSON contracts are documented, redaction/failure-path tests pass, and every production operation used by CI is reproducible from the CLI.
- Pull-request gates pass; a signed image and SBOM are published once; staging and production report the same promoted image digest.
- Infrastructure can be planned without unexpected changes and a clean staging host can be rebuilt from versioned code.
- A scheduled encrypted off-host recovery set is created, verified, retained, and monitored without manual copying.
- An isolated restore of PostgreSQL and the approved private-document scope passes migration, smoke, tenant-isolation, and M0–M6 acceptance checks within the approved RPO/RTO.
- Staging deployment, failed-deployment rollback, and production promotion have timestamped release evidence and no unreviewed manual server edits.
- DNS, TLS renewal, firewall policy, secrets, patching, disk capacity, logs, metrics, and alerts are verified in staging and production.
- Runbooks identify owners and have been exercised for deploy, rollback, restore, host replacement, certificate failure, and compromised credential/image scenarios.

## Explicitly out of scope until discovery requires it

- Kubernetes, service mesh, multi-region active-active operation, and horizontal API scaling.
- A managed PostgreSQL migration solely for fashion or convenience; it must be justified by RPO/RTO, staffing, or availability requirements.
- Automatic destructive database rollback.
- Backing up identity documents beyond their approved retention window.
