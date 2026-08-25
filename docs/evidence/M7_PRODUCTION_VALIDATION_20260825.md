# M7 production validation evidence — 2026-08-25

This sanitized record covers the owner-approved single-VPS production scope at `hotelmate.ir`. SSH keys, environment secrets, application credentials, unrestricted logs, raw documents, and recovery artifacts remain excluded from Git and mode-restricted on the host.

## Acceptance scope

[ADR-0007](../adr/0007-platform-operations-decisions.md) accepts the supplied VPS as the initial production host and GitHub `staging`/`production` as protected logical promotion gates sharing that runtime. The owner explicitly accepted two temporary risks for M7 closure: recovery sets are local rather than off-host, and Alertmanager has no external delivery receiver. Neither control is reported as implemented.

## CI, supply chain, and protected promotion

| Check | Result |
| --- | --- |
| Source commit | `884b7e7d7e22ca27796f69a3bc24af300c7d2125` |
| CI | [Run 32844808717](https://github.com/TablazOrg/HotelMate/actions/runs/32844808717) passed backend, frontend, repository-security, platform-configuration, and integrated-container jobs |
| Release and deploy | [Run 32844917487](https://github.com/TablazOrg/HotelMate/actions/runs/32844917487) passed release, staging, and production jobs |
| Supply chain | API/web vulnerability gates passed; SPDX SBOMs and provenance were published; both images and attestations were keylessly signed and verified before deployment |
| Approval | Protected production deployment `6082685062` was approved after staging acceptance |
| Release | `0.7.0-884b7e7d7e22` |
| API digest | `ghcr.io/tablazorg/hotelmate/api@sha256:f807aba2f40556dbecb23951acf12f00caf930033153a8df19ee28386c754be6` |
| Web digest | `ghcr.io/tablazorg/hotelmate/web@sha256:d40622279cc3280cbeb342ac2c3d4d3ac20214ccf77986c6a948bc9a4fffd751` |

The staging job deployed those exact digests and passed the native stateful acceptance suite plus authenticated smoke. The production job then promoted the same manifest, created a verified recovery checkpoint, applied all eight migrations, and passed public/authenticated smoke. Production evidence records `startedAt=2026-08-25T12:09:57Z`, `finishedAt=2026-08-25T12:11:29Z`, and `status=succeeded`.

The GitHub job token was used only for the private GHCR pull and removed from the host on exit. A dedicated deploy key and pinned host key are stored in the protected environments; no password or long-lived package token is used by the workflow.

## Independent production verification

| Check | Result |
| --- | --- |
| CLI identity | Installed checksum-verified CLI reported commit `884b7e7d7e22…`, release `0.7.0-884b7e7d7e22`, and eight migrations |
| Smoke | Passed again at `2026-08-25T12:36:21Z`: health, readiness, metadata, authenticated login/session, and operations report |
| Runtime | API healthy on the signed digest; web and PostgreSQL running; all 12 application/monitoring containers had zero restarts and no OOM kills after stabilization |
| DNS | Root A record resolved to the production host and `www` resolved through the root name |
| HTTP/TLS | HTTP redirected to HTTPS; HTTPS negotiated HTTP/2 and returned HSTS, CSP, Permissions Policy, referrer, MIME, framing, and cross-origin headers |
| Certificate | Trusted Let's Encrypt ECDSA certificate for `hotelmate.ir` and `www.hotelmate.ir`, valid through `2026-11-23T08:42:04Z` |
| Renewal | Certbot timer enabled/active; webroot dry-run with the validated Nginx deploy hook passed |

## Host and operations

The production Ansible pass installed the release CLI and enabled local recovery/privacy timers; its full repeat pass reported `ok=40`, `changed=0`, `unreachable=0`, and `failed=0`. The host remains key-only and non-root, with default-deny UFW, only SSH/HTTP/HTTPS public, PostgreSQL and monitoring ports bound to loopback, unattended security upgrades, fail2ban, time synchronization, 2 GiB swap, and bounded Docker logs. External checks confirmed ports 5432, 9090, 9093, and 3001 are not public.

The first timer-backed backup exercise found and fixed Docker Compose discovery inside the hardened systemd `ProtectHome` sandbox. The successful rerun created and independently verified recovery set `hotelmate-20260825T123740Z`:

- PostgreSQL custom dump: 134,488 bytes; SHA-256 `c77296e79184beee5ce0644ffe44aaecaa0bd80d0c5c442a731c3ba63a1245ce`; 229 catalog entries.
- Private-upload archive: 836 bytes; SHA-256 `1d5915eb036517dac73e27d34e633ecf44ea4d962cea8acac1e23b76f09c0afe`.
- Eight applied migrations, release `0.7.0-884b7e7d7e22`, `offHost:false`.

The daily backup timer and both privacy-retention timers are enabled/active, and one execution of each service returned success. The recovery-drill timer remains disabled because an isolated provider-backed target depends on the deferred off-host recovery work. The existing isolated local drill passed the full restore/migration/smoke/tenant-isolation/acceptance path with measured RPO 268 seconds and RTO 4 seconds; the distinct-image rollback/redeploy evidence remains in the [local validation record](M7_LOCAL_VALIDATION_20260823.md).

## Observability

Prometheus, Grafana 12.1.0, Alertmanager, Loki, Alloy, node-exporter, cAdvisor, PostgreSQL exporter, and blackbox exporter are running privately. Prometheus reported all six targets up (`hotelmate-api`, external HTTPS, PostgreSQL, node, containers, and Loki), loaded 24 rules in two groups, and had zero firing rules at capture. Loki returned ready and Alloy was shipping production container logs. Prometheus, Grafana, and Alertmanager listen on loopback only.

Alertmanager intentionally uses `unconfigured-local-receiver`; no webhook or SMTP credential exists. This is the external-alert-delivery exception in ADR-0007, not successful paging evidence.

## Accepted residual risk

The verified recovery set has `offHost:false`, so it does not survive loss of the VPS or provider account. Alert evaluation works locally, but no external responder is paged. These are the only owner-approved M7 closure exceptions and remain post-M7 remediation work.
