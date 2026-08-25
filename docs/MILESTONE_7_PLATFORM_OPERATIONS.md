# M7 — Platform operations, CLI, CI/CD, backup, and infrastructure

## Outcome

M7 turns the current single-host release baseline into a reproducible operating platform. An authorized operator must be able to validate configuration, ship an immutable release, observe it, back it up, restore it, and roll it back without assembling ad-hoc commands or exposing secrets.

The implementation remains provider-neutral, while ADR-0007 accepts the supplied single VPS as the initial production topology. Kubernetes and multi-region failover are not default requirements; they should be introduced only when availability, traffic, or organizational constraints justify their cost.

## Completion status — complete with two owner-approved exceptions

M7 was accepted on 2026-08-25 for the owner-approved single-VPS production scope. [ADR-0007](adr/0007-platform-operations-decisions.md) records the selected host, domain/TLS, recovery objectives, protected promotion policy, GHCR/Cosign supply chain, secrets model, monitoring retention, and access policy.

The owner explicitly directed closure without two external resources that were not supplied:

1. **Off-host storage:** scheduled local PostgreSQL/private-upload recovery sets are created and verified, but no encrypted S3-compatible copy exists. Host/provider-account loss can destroy runtime and recovery data together.
2. **Alert delivery:** Prometheus evaluates rules and Alertmanager runs privately, but no webhook or SMTP receiver pages an external responder.

These are documented risk acceptances, not completed controls. Production preflight and Ansible remain fail-closed unless the corresponding control is configured or its owner-approved deferral is recorded.

### Production verification evidence

The sanitized production record is [M7 production validation — 2026-08-25](evidence/M7_PRODUCTION_VALIDATION_20260825.md). The historical [external staging](evidence/M7_STAGING_VALIDATION_20260823.md) and [local validation](evidence/M7_LOCAL_VALIDATION_20260823.md) records retain the earlier bootstrap, recovery-drill, and distinct-image rollback evidence.

[CI run 32844808717](https://github.com/TablazOrg/HotelMate/actions/runs/32844808717) passed all five gates for commit `884b7e7d7e22ca27796f69a3bc24af300c7d2125`. [Release and deploy run 32844917487](https://github.com/TablazOrg/HotelMate/actions/runs/32844917487) then built once, scanned, generated SPDX SBOMs/provenance, keylessly signed and verified both images/attestations, deployed the exact digests to protected staging, passed stateful acceptance/authenticated smoke, received production approval, and promoted the same manifest.

The accepted production digests are:

- API: `ghcr.io/tablazorg/hotelmate/api@sha256:f807aba2f40556dbecb23951acf12f00caf930033153a8df19ee28386c754be6`
- Web: `ghcr.io/tablazorg/hotelmate/web@sha256:d40622279cc3280cbeb342ac2c3d4d3ac20214ccf77986c6a948bc9a4fffd751`

Production deployment evidence reports `status=succeeded` from `2026-08-25T12:09:57Z` through `12:11:29Z`, including a verified local recovery checkpoint, eight migrations, readiness, and public/authenticated smoke. An independent smoke passed again at `12:36:21Z`.

The production host is Ansible-managed and converged with a repeat `ok=40 changed=0 failed=0` pass. Key-only non-root SSH, default-deny firewalling, private PostgreSQL/monitoring ports, unattended updates, fail2ban, time sync, bounded Docker logs, 2 GiB swap, DNS, trusted root/`www` TLS, HSTS/security headers, webroot renewal, and the Nginx deploy hook are active.

Recovery set `hotelmate-20260825T123740Z` was created by the hardened systemd timer path and independently verified: a 134,488-byte PostgreSQL custom dump with 229 catalog entries, an 836-byte private-upload archive, eight migrations, and matching SHA-256 hashes. It records `offHost:false` under the approved exception. Daily backup and both privacy-purge timers are enabled and their manual service exercises passed. The isolated local drill measured RPO 268 seconds and RTO 4 seconds; the provider-backed drill timer remains disabled until the off-host exception is remediated.

The private monitoring profile is running in production. Prometheus reported all six targets up, loaded 24 alert rules, and had zero firing rules at capture; Grafana, Loki, Alloy, Alertmanager, host/container/PostgreSQL exporters, and the external HTTPS probe were ready. Prometheus/Grafana/Alertmanager bind to loopback. Alertmanager intentionally retains a local no-op receiver under the approved alert-delivery exception.

### Requirement and acceptance evidence matrix

| M7 capability | Repository evidence | Accepted status |
| --- | --- | --- |
| Single operations contract | `backend/cmd/hotelmate`, reusable `internal/operations` and `internal/acceptance`, thin Make/script aliases, unit/failure-path tests | Complete; local, staging, and production JSON/smoke paths proven |
| CI and supply chain | Pinned CI/release workflows, scans, SBOMs, provenance, Cosign verification, immutable manifest | Complete; CI 32844808717 and release 32844917487 passed |
| Delivery and rollback | Digest preflight, locks, migrations, checkpoint, activation, smoke, automatic/application rollback, evidence | Complete; protected signed staging-to-production promotion passed; distinct-image rollback/redeploy and failure paths are proven |
| Backup and restore | Coordinated manifests, checksums, catalog checks, upload safety, restic/retention interface, metrics/timers | Complete under exception; production local schedule/verification passed, off-host copy deferred |
| Recovery rehearsal | Isolated-target guard, point-in-time selection, restore, migrations, smoke, acceptance, measured RPO/RTO | Complete for implemented local path; provider-isolated scheduled drill follows the off-host remediation |
| Host baseline | Ansible firewall/access/updates/swap/log/TLS/protected directories/checksum controls | Complete; clean bootstrap, reboot persistence, TLS renewal, and zero-drift repeat proven |
| Observability | Private metrics/logs/dashboard/probes/exporters and 24 alert rules | Complete under exception; all targets up and local evaluation proven, external delivery deferred |
| Owner governance | Accepted ADR, protected environments, deployment/recovery/incident runbooks, approval evidence | Complete for the accepted single-VPS scope |

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

## Discovery decision record

These are product/operations decisions, not values the implementation should guess:

1. Hosting provider, region, data-residency constraint, monthly budget, and account owner.
2. Production and staging domains, DNS provider, certificate owner, and whether IPv6 is required.
3. Availability target, expected hotel/guest/request volume, maintenance-window policy, and scaling trigger.
4. Recovery point objective (RPO), recovery time objective (RTO), backup retention, legal holds, and whether private identity documents are recoverable or intentionally ephemeral.
5. Required environments and promotion policy: pull request, staging, production, emergency hotfix, and approval owners.
6. Container registry, secrets manager, object-storage provider, encryption-key owner, and key-rotation policy.
7. Monitoring/logging provider, data retention, alert destinations, primary responder, and escalation path.
8. SSH and administrator access model, least-privilege roles, MFA requirements, break-glass process, and access-review cadence.

The owner-approved values and the two explicit temporary exceptions are captured in accepted [ADR-0007](adr/0007-platform-operations-decisions.md). Provider billing/residency metadata remains in the owner account rather than the repository.

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

M7 is complete for the ADR-0007 scope because the following evidence exists:

- The CLI help and JSON contracts are documented, redaction/failure-path tests pass, and every production operation used by CI is reproducible from the CLI.
- Pull-request gates pass; a signed image and SBOM are published once; staging and production report the same promoted image digest.
- Infrastructure can be planned without unexpected changes and a clean staging host can be rebuilt from versioned code.
- A scheduled local recovery set is created and independently verified through the hardened timer path; encrypted off-host replication is the explicit owner-approved exception.
- An isolated restore of PostgreSQL and the approved private-document scope passes migration, smoke, tenant-isolation, and M0–M6 acceptance checks within the approved RPO/RTO.
- Staging deployment, failed-deployment rollback, and production promotion have timestamped release evidence and no unreviewed manual server edits.
- DNS, TLS renewal, firewall policy, secrets, patching, disk capacity, logs, metrics, external probes, and local alert evaluation are verified; external alert delivery is the explicit owner-approved exception.
- Runbooks identify ownership and the deploy, rollback, restore, clean-host bootstrap, certificate, and compromised credential/image paths; the executed evidence is linked above.

## Explicitly out of scope until discovery requires it

- Kubernetes, service mesh, multi-region active-active operation, and horizontal API scaling.
- A managed PostgreSQL migration solely for fashion or convenience; it must be justified by RPO/RTO, staffing, or availability requirements.
- Automatic destructive database rollback.
- Backing up identity documents beyond their approved retention window.
