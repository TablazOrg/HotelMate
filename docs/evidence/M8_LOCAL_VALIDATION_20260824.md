# M8 local validation evidence — 2026-08-24

## Result

Milestone 8 release `0.8.0-local` passed build, unit/HTTP, PostgreSQL lifecycle, migration, release preflight, deployment smoke, in-container stateful acceptance, and deployed mobile-UI checks.

## Release identity

- API image: `hotelmate-api:dev@sha256:b3310ac8698dfb81eec2b6abe6b65b804673cafa3e7750fe5e76ffd56251ea5e`
- Web image: `hotelmate-web:dev@sha256:abc4dd64671608da5a05fc37c764826b63a5945c26554b277788a515987a2e0b`
- Migration set ends at `2026082308_arrival_readiness`.
- Current release ledger: `.hotelmate/evidence/development/current-release.json`
- Successful deployment evidence: `.hotelmate/evidence/development/20260824T045502.025205000Z-succeeded.json`

## Automated verification

| Check | Evidence |
| --- | --- |
| Go unit and HTTP suite | `GOTOOLCHAIN=local go test ./...` passed every package. |
| PostgreSQL arrival lifecycle | `TestArrivalJourneyInvitationCorrectionAndReadinessPostgres` passed against Postgres 16. It covers fail-closed controls, invitation/recovery, companion evidence, terms rotation/re-signing, tenant isolation, idempotency, correction/resubmit, approval, arrival-pending, room-ready, analytics, physical check-in, expiry, and revocation. |
| Recovery regression | `TestRecoveryExchangeHashesOnlyTheSelectedCapability` proves recovery exchange does not hash/select an empty invitation token. |
| Evidence safety | Document tests reject active PDF constructs and the EICAR signature while preserving private shared-volume permissions. |
| Frontend production build | TypeScript project build and Vite production bundle passed; final JS was 319.23 kB (88.94 kB gzip) and CSS was 59.21 kB (10.54 kB gzip). |
| OpenAPI | YAML parsed successfully as OpenAPI 3.1 with 74 paths and 55 schemas; all M8 guest/staff routes are represented. |
| Release preflight | Configuration, Compose, API image, web image, and local image access checks passed. |
| Deployment smoke | `/healthz`, `/readyz`, and `/api/v1` returned 200 through `http://127.0.0.1:48080`. |
| Stateful acceptance | The CLI embedded in the deployed API container passed against `http://127.0.0.1:8080`; acceptance tenants were `acceptance-20260824045515-85ae2a57` and `acceptance-other-20260824045515-85ae2a57`. |
| Deployed UI | `/check-in` rendered as an RTL recovery form at a 390 × 844 viewport with no horizontal overflow (`scrollWidth=innerWidth=390`) and no browser console errors. Link, invitation input, recovery input, and submit control are native focus targets. |

The stateful acceptance flow covers disabled controls, administrator enablement, signed invitation creation, token exchange, a single-guest cancellation and replay denial, a companion journey, partial details, multiple evidence files, unsafe-file rejection, versioned signature, resume, idempotent replay, cross-tenant denial, reasoned document/signature correction, resubmit, approval, arrival-pending, room-ready, analytics, cross-tenant invitation denial, revocation/recovery denial, pre-arrival paid entitlement, in-room service denial before arrival, and physical check-in closure.

## Deployment incident and resolution

The first release attempt correctly recorded `failed_rolled_back`: local port `8080` was owned by an unrelated host API, so the smoke client received that process's 404 responses. Lower alternate ports were also occupied by earlier verification stacks. The M8 release was assigned free ports `48080` (API) and `43000` (web), preflight was repeated, and subsequent guarded deployments succeeded. No failed attempt was relabeled as successful.

## Deliberately open general-availability evidence

The automated/local result does not claim completion of moderated participant research, product-owner target approval, manual screen-reader review, or an external production promotion. Those gates are listed in [M8 implementation and operation](../MILESTONE_8_ONLINE_CHECKIN.md) and remain dependent on people or the M7 production platform.
