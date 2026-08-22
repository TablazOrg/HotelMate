# M7 local validation evidence — 2026-08-23

This is a sanitized, reviewable record of the local M7 exercise. Raw JSON, database dumps, private-upload archives, generated credentials, and environment files remain under the ignored mode-restricted `.hotelmate/` workspace and must not be committed. Timestamps below are UTC on 2026-08-22; the operator timezone was Asia/Tehran on 2026-08-23.

## Scope and result

| Check | Result |
| --- | --- |
| Current local deploy | `hotelmate deploy apply --yes` succeeded from `hotelmate.release/v1` at `21:31:23Z` |
| Edge readiness | `GET http://localhost:3000/readyz` returned `ready` |
| Authenticated smoke | Health, readiness, API metadata, staff login/session, and operations report passed |
| Native Go acceptance | Passed after replacing the shell implementation; generated tenants were `acceptance-20260822210328-91565502` and `acceptance-other-20260822210328-91565502` |
| Schema | Seven migrations applied; zero pending |
| Release identity | `hotelmate_build_info{version="0.7.0-local",commit="working-tree",image="hotelmate-api:dev"} 1` |
| Running API image | Local OCI image ID `sha256:97857d6179926176c7ea2f6a24f1a6ada8f53cd5ae214f5159d8209203479963` |
| Application rollback | A prior distinct-image rollback, smoke verification, and redeploy succeeded through the CLI evidence path |
| Main-branch CI | [CI #21](https://github.com/TablazOrg/HotelMate/actions/runs/32600378218) passed backend, frontend, repository-security, platform-configuration, and container gates for commit `fb5dbd8` |
| Immutable release | The release job in [Release and deploy #5](https://github.com/TablazOrg/HotelMate/actions/runs/32600447283) published both GHCR images, passed high/critical scans, generated SPDX SBOMs, signed and verified images/attestations, and uploaded `hotelmate-release-0.7.0-fb5dbd8ade4c` (5.1 MB; artifact SHA-256 `51cd68a3b20031bb145ea7510002b3df9d910b43a7d9ac9338ed95c1c03a6b4c`) |
| External promotion | Staging reached the exact-digest SSH deployment step, then failed in seven seconds with exit 255 because no deployment-capable external target/credentials were supplied; production was correctly skipped |

The acceptance suite covered tenant onboarding/default content, primary-admin and housekeeping RBAC, cross-tenant reservation/request/document denial, reservation and stay transitions, pre-arrival and active-stay ordering, private document upload/download integrity, prompt-injection handoff, department fulfillment, reports, correlated audits, checkout, and session invalidation.

## Recovery set

Recovery set: `hotelmate-20260822T203931Z`.

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| PostgreSQL custom dump | 125,940 | `871499bc3dceeed64f6a96bccf6ac77f5536bf92d6f4d36c009d4103fe987676` |
| Private-upload archive | 989 | `af0156b4312138fdb1aeb286b686c9e7d918e8e1d2f6b0405035a3a5991546b0` |
| Recovery manifest | — | `a202b77a8b0af9a2d8343157471418b68fa027a9bf22c4c4f3407ed0d4c62cd6` |

`backup verify` recomputed both artifact hashes, rejected unsafe manifest/path forms in automated tests, and obtained 162 entries from the PostgreSQL restore catalog. The manifest recorded all seven migration ledger entries. This development recovery set was deliberately local (`offHost: false`) and therefore does not satisfy the production off-host gate.

## Isolated recovery drill

The `hotelmate backup drill --yes` target used a dedicated non-production database, a separate uploads path, an isolated API container on loopback, and `HOTELMATE_ISOLATED_RECOVERY_DRILL=true`. The source set was selected at or before the requested recovery point.

| Measurement | Result |
| --- | ---: |
| Requested recovery point | `2026-08-22T20:44:00.057851Z` |
| Source recovery set | `hotelmate-20260822T203931Z` |
| Verification catalog | 162 entries |
| Measured local RPO | 268 seconds |
| Measured local RTO | 4 seconds |
| Restore/migrations | Passed; seven migrations applied |
| Public/authenticated smoke | Passed |
| Stateful acceptance and tenant isolation | Passed |
| Upload comparison | No differences |
| Cleanup | Temporary API container and exact drill database absent after completion |

The successful drill result had SHA-256 `c7977526cf29b063f235993613693bd3b6e8a8938d0a0a28e7415fb434e0a516`. The textfile collector metric ended with `hotelmate_job_last_run_success{job="restore_drill"} 1` and matching last-run/last-success timestamps.

## Interpretation and external gates

This exercise proves the provider-neutral implementation, local safety path, and remote supply-chain path through signed, attested, digest-pinned GHCR artifacts. It does not prove approved production RPO/RTO, public DNS/TLS, encrypted restic transfer, storage immutability, protected production approval, provider rebuild/drift, paging delivery, or staging/production availability. Those require the owner inputs in [ADR-0007](../adr/0007-platform-operations-decisions.md) and real external resources.
