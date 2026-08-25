# ADR-0007: Platform operations decisions

- Status: Accepted
- Date: 2026-08-25
- Scope: M7 staging and production operations
- Approver: repository owner and protected-production reviewer `naderh232`

## Context

M7 required external hosting, recovery, access, and ownership decisions before the provider-neutral CLI, release pipeline, recovery contract, Compose runtime, Ansible baseline, and observability stack could be treated as production operations. The owner supplied a production VPS and `hotelmate.ir`, authorized deployment, approved the defaults below, and explicitly directed M7 to close while two controls remain deferred: off-host backup replication and external alert delivery.

## Accepted decisions

| Decision | Accepted value |
| --- | --- |
| Hosting and topology | The owner-supplied Ubuntu 24.04 VPS is the initial low-volume production host. GitHub `staging` and `production` are separate protected promotion gates but share this single runtime because only one host was supplied. Ansible is the rebuild source of truth. Provider account, billing, and exact residency metadata remain owner-held account records. |
| Domain and TLS | `hotelmate.ir` and `www.hotelmate.ir` use WebRamz authoritative DNS, IPv4, and Let's Encrypt. Certbot uses a mounted webroot and a validated Nginx reload hook. |
| Availability and capacity | Initial objective: 99.9% monthly external HTTPS availability, 99% of API requests below one second, and fewer than 2% 5xx responses over ten minutes. Upgrade or separate the topology when sustained resource use exceeds 75%, the SLO is missed, or operational load requires isolation. Maintenance is owner-coordinated. |
| Recovery policy | RPO 24 hours, RTO 2 hours, 14 daily and 8 weekly recovery points. PostgreSQL and in-retention private uploads are one checksummed recovery set. Local daily creation, verification, and privacy timers are active. Host-loss durability is not met until the off-host exception below is remediated. |
| Promotion and approval | Main-branch CI must pass; release images are built once, scanned, SBOMed, keylessly signed, and verified. The same digest is deployed to protected `staging`, accepted, manually approved by `naderh232`, then promoted to protected `production`. Emergency releases use the same integrity and smoke gates. |
| Registry and secrets | Private GHCR with GitHub job-scoped pull authorization and keyless Cosign. GitHub environment secrets hold deploy/smoke material; the VPS uses a mode-`600` protected environment. The deployment identity is a dedicated key-only account. |
| Metrics and logs | Private Prometheus/Grafana/Alertmanager/Loki/Alloy on the production host, 30-day Prometheus retention, external HTTPS probes, dashboard provisioning, and 24 alert rules. Alertmanager remains local-only under the exception below. |
| Administrative access | Dedicated non-root `hotelmate` user, key-only SSH, root/password/keyboard-interactive login disabled, default-deny firewall, and passwordless automation limited to the approved operator. Provider-console access is the break-glass path; review keys and GitHub environment access quarterly and after personnel changes. |

## Owner-approved temporary exceptions

1. **Off-host recovery storage:** no S3-compatible endpoint or restricted credentials were supplied. Production continues to create and verify local PostgreSQL/private-upload recovery sets, but a host or provider-account loss can destroy runtime and backups together. Add restricted encrypted object storage, retention/immutability, and a provider-isolated drill after M7.
2. **External alert delivery:** no Alertmanager webhook or SMTP app password was supplied. Metrics, logs, dashboards, probes, and rules run locally, but alerts do not page an operator. GitHub deployment notifications and manual dashboard review are the interim detection paths. Add and exercise an approved receiver after M7.

Both exceptions are explicit production risk acceptances, not claims that the controls exist. The repository and Ansible preflight require a configured control or this recorded owner deferral.

## Consequences

M7 is accepted for the owner-approved single-VPS production scope. CI and supply-chain gates, protected promotion, immutable digests, production recovery checkpoints, local scheduled recovery and privacy jobs, DNS/TLS renewal, host hardening, metrics/logs/dashboard/rules, smoke/acceptance, and the tested recovery/rollback command paths are operational.

The system is not resilient to loss of the production host/account, and an actionable alert will not reach an external responder until the two exceptions are remediated. Multi-host staging, managed databases, Kubernetes, and multi-region failover remain future capacity/availability decisions rather than M7 requirements.
