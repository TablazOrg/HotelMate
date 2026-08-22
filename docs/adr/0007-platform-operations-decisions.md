# ADR-0007: Platform operations decisions

- Status: Proposed — owner approval required
- Date: 2026-08-22
- Scope: M7 staging and production operations

## Context

M7 requires external commercial, legal, recovery, access, and ownership choices that cannot be inferred safely from source code. The repository now implements a provider-neutral CLI, immutable release pipeline, restic recovery-set contract, Compose runtime, Ansible host baseline, and observability stack. Those components deliberately fail preflight rather than fabricate missing production policy.

## Decisions awaiting approval

| Decision | Required owner value | Current repository behavior |
| --- | --- | --- |
| Provider, region, residency, budget, account owner | Pending | No provider resource is created; Ansible begins from an approved Ubuntu host |
| Staging/production domains, DNS, certificate owner, IPv6 | Pending | TLS Compose and external probe require supplied domains/certificates |
| Availability, volume, maintenance, scaling trigger | Pending | Proposed SLO only; one API replica remains enforced by architecture |
| RPO, RTO, retention, legal hold, document recovery | Pending | Daily logical recovery set and 14 daily/8 weekly restic proposal; not approved |
| Environments, promotions, hotfix, approvers | Pending | GitHub `staging` then protected `production`; environment reviewers must be configured |
| Registry, secrets manager, object storage, key owner/rotation | Pending | GHCR/keyless cosign/restic interfaces implemented; accounts and rotation owners absent |
| Metrics/log provider, retention, alert destinations, responders | Pending | Self-hosted 30-day metrics proposal; alert receiver intentionally unconfigured |
| SSH/admin roles, MFA, break glass, access review | Pending | Key-only non-root SSH baseline; approved keys, MFA/break-glass process, and cadence absent |

## Implemented reversible defaults

- GitHub Actions builds API/web once, scans, produces provenance and SPDX SBOMs, signs and verifies with cosign, deploys staging, then promotes the exact digests through the protected production environment.
- PostgreSQL and approved private uploads form one checksummed recovery set. Restic encrypts before off-host transfer and enforces retention.
- Ubuntu/Docker/Compose with Ansible is the current single-host baseline. Ports 80/443 and approved SSH are the only inbound firewall allowances; PostgreSQL binds to host loopback only.
- Prometheus/Grafana/Alertmanager run privately on the host. External managed telemetry may replace them after approval without changing application metrics.

## Consequences

M7 implementation can be built, tested, deployed, rolled back, and restored locally now. The local isolated drill measured RPO at 268 seconds and RTO at 4 seconds, but those values are evidence of the implementation rather than approved production objectives. A real staging/production deployment, successful encrypted off-host transfer, scheduled provider-backed drill against approved RPO/RTO, tested paging route, DNS/TLS renewal, and provider rebuild/drift evidence remain acceptance blockers until this ADR is approved and credentials/resources are available.
