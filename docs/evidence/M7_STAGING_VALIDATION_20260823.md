# M7 external staging validation evidence — 2026-08-23

This is a sanitized record of the deployment to the operator-supplied VPS. The host address, SSH key material, generated application/database credentials, protected environment, raw private documents, and unrestricted logs are intentionally excluded. Raw JSON evidence and recovery artifacts remain mode-restricted on the host.

## Scope and classification

The supplied host is classified as **staging**, not production. No approved domain, DNS ownership, trusted certificate, GHCR pull authorization, off-host restic repository, production approver, recovery objectives, secrets backend, or paging receiver was supplied. The service therefore runs over HTTP for validation; HSTS is disabled; observability and scheduled jobs that depend on those external decisions remain disabled.

## Host baseline

| Check | Result |
| --- | --- |
| Operating system | Ubuntu 24.04.4 LTS, x86_64, one vCPU |
| Capacity at evidence capture | 961 MiB RAM, 2 GiB managed swap, 19 GiB root volume with 13 GiB free |
| Runtime tooling | Docker 29.1.3, Buildx 0.30.1, Compose 2.40.3 |
| Access | Dedicated `hotelmate` operator; key-only SSH; root, password, keyboard-interactive, and X11 SSH disabled |
| Network | UFW active, default-deny inbound; only SSH, HTTP, and HTTPS allowed; PostgreSQL published on loopback only |
| Host services | Docker, fail2ban, time synchronization, unattended security updates, and certbot renewal enabled/active as applicable |
| Artifact verification | Release CLI SHA-256 verified before installation; Cosign v3.1.2 matched the upstream checksum |
| Configuration convergence | Initial bootstrap completed without failures; the final repeat playbook run reported `changed=0`, `failed=0` |
| Reboot persistence | Host reboot completed; key-only access, swap, firewall, Docker, fail2ban, and runtime services returned successfully |

The Ansible baseline also installs bounded Docker JSON logs, protected configuration/recovery directories, systemd unit definitions, and an automatically rendered external probe target. Production configuration is fail-closed when the alert receiver is not approved.

## Runtime deployment

The final staging API and operations CLI were built from pushed commit `1e70f05e5c5ef5f1c85eb48049c9bf750a18cd4b` with release identity `0.7.0-1e70f05e5c5e`. Private GHCR artifacts were not anonymously pullable, so the host did not bypass the Cosign gate or impersonate a signed promotion. Instead, source-identical Linux/amd64 artifacts were built on the trusted controller, transferred over key-only SSH, and configured with `pull_policy=never` for this staging exercise only. Production retains `pull_policy=always` by default.

| Component | Runtime evidence |
| --- | --- |
| API | Healthy, unprivileged UID 100 with the operator group, immutable local image ID `sha256:a4d1f4d19e3f7e9df9df36ba6f2c7f6aa113d6a74d2aba21df81a85a9e97f7e9` |
| Web edge | Running on public HTTP with immutable local image ID `sha256:2bb5940e1a637ede869f72a6712f347f4dd766239b0e3dfd80c0f7ce87f9a489` |
| PostgreSQL | Healthy on pinned `postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777`; host port bound to `127.0.0.1` |
| Schema | Seven migrations applied, zero pending |
| External edge | `/healthz`, `/readyz`, `/api/v1`, and the web root returned HTTP 200 with the configured security headers |

The deployment exercise found and fixed a real cross-boundary issue: API-created private-document paths were inaccessible to the non-root backup operator. The API now writes group-restricted `0770` directories and `0660` files, Compose runs it with the resolved operator GID, restore extraction preserves that contract, and Ansible reconciles existing paths. Unit tests assert the shared private modes. A new remote acceptance document proved that both the API and backup operator can use the volume while access remains closed to other users.

## Application and recovery verification

| Check | Result |
| --- | --- |
| Authenticated smoke | Passed at `2026-08-23T17:38:26Z`: health, readiness, metadata, login, session, and operations report |
| Stateful acceptance after exact-commit deployment | Passed at `2026-08-23T17:38:33Z`, including onboarding, RBAC, tenant isolation, lifecycle, ordering, private document integrity/access, AI handoff, fulfillment, reports/audits, checkout, and session expiry |
| Recovery set | `hotelmate-20260823T173844Z` created and verified |
| PostgreSQL dump | 88,122 bytes; SHA-256 `b11a47044aafb62b56e584beb50e4b637a160c231e5afdeb68ade5b8844ae252` |
| Private-upload archive | 453 bytes; SHA-256 `c7271db3dad6e91aebca50b829d7f6f1dddb4ed96fa873b5b4cce5bf91faad70` |
| Recovery verification | Both hashes matched, `pg_restore` reported 162 catalog entries, and all seven migrations were present |

The staging recovery set deliberately records `offHost: false`. It proves coordinated local creation and verification but is not durable against host/account loss and cannot satisfy the production backup gate.

## Expected incomplete checks

`hotelmate doctor` passed configuration, Docker, daemon, Cosign, disk headroom, database, target URL/DNS/connectivity, Compose, and release-manifest checks. Its two registry checks correctly failed because the release images require GHCR authorization and the staging exercise uses controller-loaded local image IDs. No signature, registry, or production backup guard was disabled.

[CI #26](https://github.com/TablazOrg/HotelMate/actions/runs/32654977500) passed backend, frontend, repository-security, platform-configuration, and integrated-container jobs for the exact deployed commit.

The release job in [Release and deploy #8](https://github.com/TablazOrg/HotelMate/actions/runs/32655051216) then built and published the exact-commit API/web images, passed high/critical scans, generated SPDX SBOMs and provenance, and signed and verified the images and attestations. Its downstream GitHub staging job failed in five seconds with SSH exit 255 because the repository's `staging` environment still has no deployment SSH configuration; production was correctly skipped. The separately authorized controller deployment documented above is therefore the staging runtime evidence.

The following remain explicit M7 blockers:

- approved domain/DNS and trusted TLS issuance/renewal evidence;
- registry authorization followed by Cosign-verified signed-image deployment through GitHub staging and protected production environments;
- encrypted off-host restic storage, retention/immutability, scheduled backup, and provider-isolated recovery drill against approved RPO/RTO;
- approved Alertmanager receiver, central observability retention, alert delivery/failure exercises, and an operations owner;
- production secrets manager, access-review/break-glass policy, capacity/SLO approval, production promotion, and rollback evidence.

This record proves a hardened external staging deployment and local-on-host recovery verification. It does not claim production readiness or M7 completion.
