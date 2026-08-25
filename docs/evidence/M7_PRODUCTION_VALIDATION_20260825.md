# M7 production validation evidence — 2026-08-25

This sanitized record covers the owner-approved single-VPS production scope at `hotelmate.ir`. SSH keys, environment secrets, application credentials, unrestricted logs, raw documents, and recovery artifacts remain excluded from Git and mode-restricted on the host.

## Acceptance scope

[ADR-0007](../adr/0007-platform-operations-decisions.md) accepts the supplied VPS as the initial production host and GitHub `staging`/`production` as protected logical promotion gates sharing that runtime. The owner explicitly accepted two temporary risks for M7 closure: recovery sets are local rather than off-host, and Alertmanager has no external delivery receiver. Neither control is reported as implemented.

## CI, supply chain, and protected promotion

| Check | Result |
| --- | --- |
| Source commit | `e13acc62d0b5bb818c5d2a01963451a6a1869e80` |
| CI | [Run 32857623164](https://github.com/TablazOrg/HotelMate/actions/runs/32857623164) passed backend, frontend, repository-security, platform-configuration, and integrated-container jobs |
| Release and deploy | [Run 32857767399](https://github.com/TablazOrg/HotelMate/actions/runs/32857767399) passed release, staging, and production jobs |
| Supply chain | API/web vulnerability gates passed; SPDX SBOMs and provenance were published; both images and attestations were keylessly signed and verified before deployment |
| Approval | Protected production deployment `6085114896` was approved after staging acceptance |
| Release | `0.7.0-e13acc62d0b5` |
| API digest | `ghcr.io/tablazorg/hotelmate/api@sha256:bf14969a122467e488d301956926f6e8742ba82018baf2d4e8b8d0ed8baa6e1b` |
| Web digest | `ghcr.io/tablazorg/hotelmate/web@sha256:bbeb49b6fc33f237b4845a4c211ad591e20c10fd10baac4dd9d1103c4e5e7aab` |

The staging job deployed those exact digests and passed the native stateful acceptance suite plus authenticated smoke. The production job then promoted the same manifest, created a verified recovery checkpoint, applied all eight migrations, and passed public/authenticated smoke. Production evidence records `startedAt=2026-08-25T14:19:42Z`, `finishedAt=2026-08-25T14:22:13Z`, and `status=succeeded`.

The GitHub job token was used only for the private GHCR pull and removed from the host on exit. A dedicated deploy key and pinned host key are stored in the protected environments; no password or long-lived package token is used by the workflow.

## Independent production verification

| Check | Result |
| --- | --- |
| CLI identity | Installed CLI SHA-256 `4343fada60851bfda194eb35dcc6e4a71b2c5dda2238f53de562c16d45d390e2` matched the release bundle for commit `e13acc62d0b5…`, release `0.7.0-e13acc62d0b5`, and eight migrations |
| Smoke | The protected production job passed health, readiness, metadata, authenticated login/session, and operations report; independent public health/readiness/version checks passed again through `2026-08-25T14:38Z` |
| Runtime | API healthy on the signed digest; web and PostgreSQL running; all 12 application/monitoring containers had zero restarts and no OOM kills after stabilization |
| DNS | Root A record resolved to the production host and `www` resolved through the root name |
| HTTP/TLS | HTTP redirected to HTTPS; HTTPS negotiated HTTP/2 and returned HSTS, CSP, Permissions Policy, referrer, MIME, framing, and cross-origin headers |
| Certificate | Trusted Let's Encrypt ECDSA certificate for `hotelmate.ir` and `www.hotelmate.ir`, valid through `2026-11-23T08:42:04Z` |
| Renewal | Certbot timer enabled/active; webroot dry-run with the validated Nginx deploy hook passed |

## Host and operations

The production Ansible pass installed the final release CLI and rendered the approved external probe target; its full repeat from the same final inputs reported `ok=39`, `changed=0`, `unreachable=0`, and `failed=0`. The host remains key-only and non-root, with default-deny UFW, only SSH/HTTP/HTTPS public, PostgreSQL and monitoring ports bound to loopback, unattended security upgrades, fail2ban, time synchronization, 2 GiB swap, and bounded Docker logs. External checks confirmed ports 5432, 9090, 9093, and 3001 are not public.

The first timer-backed backup exercise found and fixed Docker Compose discovery inside the hardened systemd `ProtectHome` sandbox. The final post-promotion exercise created and independently verified recovery set `hotelmate-20260825T143658Z`:

- PostgreSQL custom dump: 165,899 bytes; SHA-256 `30e09cd59566b38746e43e1a54d7d2de4a86c9f8d2b2d1dc2cd0f6e76c3d74cb`; 229 catalog entries.
- Private-upload archive: 1,417 bytes; SHA-256 `1521abeecfdabdc921a0969c24ccd548dc95b67eb91e95388d4e683031193807`.
- Eight applied migrations, release `0.7.0-e13acc62d0b5`, `offHost:false`.

The daily backup timer and both privacy-retention timers are enabled/active, and one execution of each service returned success. The recovery-drill timer remains disabled because an isolated provider-backed target depends on the deferred off-host recovery work. The existing isolated local drill passed the full restore/migration/smoke/tenant-isolation/acceptance path with measured RPO 268 seconds and RTO 4 seconds; the distinct-image rollback/redeploy evidence remains in the [local validation record](M7_LOCAL_VALIDATION_20260823.md).

## Observability

Prometheus, Grafana 12.1.0, Alertmanager, Loki, Alloy, node-exporter, cAdvisor, PostgreSQL exporter, and blackbox exporter are running privately. Prometheus reported all six targets up (`hotelmate-api`, `https://hotelmate.ir`, PostgreSQL, node, containers, and Loki) and loaded 24 rules in two groups. No unexpected alert was active at final capture. `HotelMateRestoreDrillStale` remains firing by design as the visible indicator that the owner-approved off-host/provider-isolated drill control is deferred. Loki returned ready and Alloy was shipping production container logs. Prometheus, Grafana, and Alertmanager listen on loopback only.

The final consecutive-promotion check exposed that application-only Compose orphan cleanup could stop the separately started monitoring project. Deployment composition now automatically includes the sibling observability definition in managed staging/production environments, so activation and rollback preserve—and on a new host start—the monitoring services. A regression test keeps development application-only while requiring the combined managed project.

The same closure check found Prometheus renaming the operations CLI textfile collector's `job` label to `exported_job`, which made the backup-freshness rule report an absent series despite a successful current backup. The node-exporter scrape now honors the collector labels, restoring the backup, purge, deploy, migration, and drill alert/dashboard queries to their intended series.

Alertmanager intentionally uses `unconfigured-local-receiver`; no webhook or SMTP credential exists. This is the external-alert-delivery exception in ADR-0007, not successful paging evidence.

## Accepted residual risk

The verified recovery set has `offHost:false`, so it does not survive loss of the VPS or provider account. Alert evaluation works locally, but no external responder is paged. These are the only owner-approved M7 closure exceptions and remain post-M7 remediation work.
