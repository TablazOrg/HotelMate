# M7 — Platform operations, CLI, CI/CD, backup, and infrastructure

## Outcome

M7 turns the current single-host release baseline into a reproducible operating platform. An authorized operator must be able to validate configuration, ship an immutable release, observe it, back it up, restore it, and roll it back without assembling ad-hoc commands or exposing secrets.

The target remains intentionally provider-neutral until the discovery decisions below are approved. Kubernetes and multi-region failover are not default requirements; they should be introduced only when availability, traffic, or organizational constraints justify their cost.

## Implementation status — external staging deployed, production gates pending

As of 2026-08-23, the repository contains the M7 provider-neutral foundation:

- A tested Go `hotelmate` CLI with protected configuration precedence, secret redaction, `hotelmate.operations/v1` JSON, stable exit codes, mandatory confirmation for every mutation, signal-aware ownership locks, job metrics, migrations, recovery sets and drills, guarded retention, deploy/rollback evidence, authenticated smoke, a native Go acceptance suite, and release identity.
- Digest-pinned production Compose plus pinned GitHub workflows that build once, scan API/web images, generate SBOMs and provenance, sign and verify images/attestations, deploy staging, and promote the same manifest through the protected production environment. CI also syntax-checks Ansible and validates production/observability configuration.
- Coordinated PostgreSQL/private-upload `hotelmate.recovery-set/v1` manifests with strict parsing, path safety, SHA-256 and PostgreSQL catalog verification, encrypted restic off-host transfer/retention, daily schedules, job-freshness metrics, and an automated isolated `hotelmate.recovery-drill/v1` workflow.
- A provider-neutral Ubuntu/Ansible baseline for non-root deployment, key-only SSH, firewalling, unattended security updates, Docker log rotation, certificate renewal, protected directories, timers, and checksummed CLI/cosign installation.
- Prometheus application/release metrics, `pg_stat_statements`, PostgreSQL/host/container/external probes, Loki/Alloy request-ID correlation, a provisioned Grafana dashboard, and alerts for availability, errors, latency, authentication, WebSockets, operations, TLS, capacity, database behavior, backups, purges, and restore-drill age.

M7 remains **in progress**, not complete. An operator-supplied Ubuntu VPS has now been hardened and validated as staging, but [ADR-0007](adr/0007-platform-operations-decisions.md) is unapproved and the host came without a domain/DNS owner, trusted TLS certificate, registry authorization, approved production protection, off-host restic destination, secrets manager, recovery objectives, or paging receiver. It cannot truthfully be treated as production or prove protected production promotion, encrypted off-host durability, approved recovery objectives, or routed-alert evidence.

### External staging verification evidence

The sanitized record is [M7 external staging validation — 2026-08-23](evidence/M7_STAGING_VALIDATION_20260823.md). The supplied host was bootstrapped through Ansible, rebooted, and converged to a zero-change repeat run. Key-only non-root SSH, default-deny UFW, private PostgreSQL, bounded Docker logs, unattended updates, fail2ban, time synchronization, a 2 GiB swap file, and checksum-verified CLI/Cosign installation were verified.

All seven migrations, public/authenticated smoke, and the native stateful acceptance suite passed remotely. A staging recovery set containing PostgreSQL plus the private-upload scope was created and independently verified with matching hashes and 162 PostgreSQL catalog entries. The exercise also exposed and fixed the API/backup-operator upload permission boundary; the shared group contract is now enforced in the application, Compose, restore path, tests, and Ansible.

This staging deployment intentionally uses HTTP, controller-loaded immutable Linux/amd64 images, and a local-only recovery set. Private GHCR access was unavailable, so registry/signature gates were not bypassed; scheduled operations and the resource-heavy observability profile remain disabled until off-host storage and an approved alert receiver exist.

[CI #26](https://github.com/TablazOrg/HotelMate/actions/runs/32654977500) passed all five gates for the exact deployed commit. The release job in [Release and deploy #8](https://github.com/TablazOrg/HotelMate/actions/runs/32655051216) published, scanned, SBOMed, signed, attested, and verified both images; its GitHub staging job then failed with SSH exit 255 because the repository environment still lacks deployment SSH configuration. Production was correctly skipped.

### Local verification evidence

The sanitized evidence record is [M7 local validation — 2026-08-23](evidence/M7_LOCAL_VALIDATION_20260823.md). In summary:

- The current `hotelmate-api:dev` and `hotelmate-web:dev` release was applied through `hotelmate deploy apply --yes`; public and authenticated smoke passed, and the API reports `0.7.0-local`, commit `working-tree`, and image `hotelmate-api:dev` through its private metrics endpoint.
- The native Go stateful acceptance suite passed through the Nginx edge and exercised onboarding, RBAC, tenant isolation, reservation/stay lifecycle, paid ordering, private documents, AI handoff, fulfillment, reporting/audits, and checkout.
- Recovery set `hotelmate-20260822T203931Z` contains a 125,940-byte PostgreSQL custom dump and a 989-byte private-upload archive. Both hashes matched, `pg_restore` reported 162 catalog entries, and all seven migrations were present.
- An isolated non-production drill restored that set into a dedicated database and upload path, ran migrations, authenticated smoke, tenant isolation, private-document checks, and the complete acceptance suite, then removed the temporary API container and database. It measured local RPO at 268 seconds and local RTO at 4 seconds.
- A distinct-image application rollback and redeploy passed previously through the same CLI evidence path.
- [CI #21](https://github.com/TablazOrg/HotelMate/actions/runs/32600378218) passed all five gates for commit `fb5dbd8`. The release job in [Release and deploy #5](https://github.com/TablazOrg/HotelMate/actions/runs/32600447283) published the API/web images to GHCR, passed high/critical image scans, generated SPDX SBOMs, signed and verified both images and attestations, and uploaded the immutable release bundle.
- The same release reached staging and failed only at the SSH deployment step with exit 255 because a deployment target and credentials were not supplied; production was correctly skipped rather than promoting an untested release.

These values prove the local command paths, validation, recovery selection, and safety guards. They are not approved production RPO/RTO targets, public availability, encrypted off-host durability, or provider rebuild evidence.

### Requirement and acceptance evidence matrix

| M7 capability | Repository evidence | Current status |
| --- | --- | --- |
| Single operations contract | `backend/cmd/hotelmate`, reusable `internal/operations` and `internal/acceptance`, thin Make/script compatibility aliases, unit/failure-path tests | Implemented; local and external staging smoke/acceptance proven |
| CI and supply chain | Pinned CI/release workflows, tests, scans, SBOMs, provenance, cosign verification, immutable release manifest | Implemented and remotely proven by CI #26/release #8; branch and production-environment enforcement pending |
| Delivery and rollback | Digest policy, preflight, owner locks, migrations, checkpoint, activation, authenticated smoke, automatic/application rollback, timestamped evidence | External manual staging deployment/acceptance proven; signed automated promotion and production rollback pending |
| Backup and restore | Strict coordinated manifests, checksums, PostgreSQL catalog checks, upload path safety, restic encryption/retention, metrics/timers | External staging local recovery set verified; approved off-host repository, schedule, and immutability pending |
| Recovery rehearsal | Isolated-target guard, point-in-time set selection, restore, migrations, smoke, native acceptance, RPO/RTO evidence, monthly timer | Local drill passed; scheduled provider-backed drill and approved objectives pending |
| Host baseline | Versioned Ansible, firewall, key-only user, updates, swap, log rotation, certbot timer, protected directories, checksum verification | Applied to an external Ubuntu host, rebooted, and converged with `changed=0`; replacement-host rebuild still pending |
| Observability | Private Prometheus/Grafana/Alertmanager/Loki/Alloy profile, release/request-ID/database/job signals, dashboard, alert rules | Implemented and statically validated; real retention, paging route, and alert exercises pending |
| Owner governance | ADR, deployment/recovery/incident runbooks, explicit external acceptance gates | Documented; owner decisions, roles, approvals, access review, and exercises pending |

## Pre-M7 discovery baseline

This table records the gaps that drove the milestone. The implementation/evidence matrix above is the current state.

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
hotelmate backup create|list|verify|restore|drill
hotelmate retention purge-documents|purge-messages
hotelmate deploy preflight|apply|status|rollback
hotelmate smoke
hotelmate acceptance
hotelmate version
```

Requirements:

- Flags override environment variables, which override an optional mode-restricted config file; resolved secrets are never printed.
- Every mutating or destructive command identifies the target environment and requires `--yes`, including development, so automation and humans share one safety contract.
- `--json` emits a versioned schema, stdout contains results, stderr contains diagnostics, and exit codes distinguish invalid configuration, failed preconditions, command failure, and verification failure.
- `doctor` checks required tools, target connectivity, DNS, TLS, Compose/registry access, disk headroom, database readiness, and backup destination access without changing state.
- CLI unit tests cover parsing and redaction; integration tests cover migrations, backup verification, restore guards, smoke checks, and failure exit codes.
- Existing Make targets and compatibility scripts remain thin aliases that call the CLI, preventing two independent operational implementations.

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
