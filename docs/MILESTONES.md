# HotelMate delivery milestones

These milestones are derived from the attached project definition. The requested Go/GORM/React/PostgreSQL stack is the implementation baseline for every milestone.

## M0 — Infrastructure baseline (complete in this change)

- Establish a Go API module with configuration, graceful shutdown, structured request logging, CORS, `/healthz`, `/readyz`, and `/api/v1` metadata endpoints.
- Establish GORM/PostgreSQL connection management and an initial domain schema foundation for hotels, guests, staff, rooms, reservations, stays, services, requests, catalog, knowledge, and conversations.
- Establish a React + TypeScript + Vite RTL shell and a small API connectivity indicator.
- Add Docker Compose for Postgres/API/web, health checks, persistent local volumes, environment template, Makefile, and contributor documentation.

## M1 — Identity, tenancy, and access control (complete)

- Add hotel onboarding and branding settings (logo, primary color, timezone).
- Implement separate JWT claims and middleware for guests and staff.
- Implement guest login using room number plus national ID/passport, with privacy-safe responses and audit logging.
- Implement staff accounts and role permissions: primary admin, secondary admin, operations manager, reception, housekeeping, and F&B.
- Add migrations and API tests for authentication, authorization, and tenant isolation.

Delivered with a versioned migration ledger, OpenAPI contract, guarded deployment onboarding, bcrypt password/identity verification, audience-separated JWTs, security audit records, role-aware staff management, branding administration, responsive RTL React flows, and database-backed Compose verification.

## M2 — Guest stay and reservation lifecycle (complete)

- Build reservation lookup and confirmed-reservation to pre-arrival stay conversion.
- Implement reception check-in/check-out and room status transitions.
- Add online check-in workflow with local document handling, validation, and retention rules.
- Deliver guest-facing pages for pre-arrival and active-stay states.

Delivered with an atomic reservation → pre-arrival → active → checked-out state machine, room availability/occupied/cleaning transitions, reservation overlap protection, tenant- and role-bound reception APIs, private MIME-sniffed document storage, SHA-256 metadata, authenticated downloads, replacement and retention handling, a purge command, PostgreSQL integration coverage, and responsive guest/reception interfaces.

## M3 — Service catalog and request operations (complete)

- Implement the six quick actions (housekeeping, bottled water, tea/coffee, amenities, late checkout, transfer) plus the full service catalog.
- Add service request creation, assignment, priority, status transitions, notes, and history.
- Build the operational queue for reception, housekeeping, and F&B with filters and role-aware actions.
- Add real-time updates between guest and staff using a WebSocket gateway; persist events for reconnect/history.

Delivered with six seeded quick actions, tenant-scoped catalog administration, active-stay guest ordering/cancellation, assignment-role validation, priority and status controls, immutable event history, department queues, tenant/stay/department-filtered WebSocket delivery, API tests, and PostgreSQL upgrade verification. The React delivery now includes the approved handoff-based Persian RTL guest home/catalog/live tracking experience, one-tap requests, mobile fulfillment queues, an actionable realtime operations dashboard, catalog activation/creation controls, and responsive tenant branding. Paid ordering and AI surfaces remain explicitly gated to M4 and M5.

## M4 — Revenue and hotel content (complete)

- Implement paid services and pre-arrival upsell flows with prices/currency and order totals.
- Add facilities, promotions, restaurants, and menu management plus guest browsing views.
- Add availability windows, activation controls, and tenant-scoped content administration.
- Keep payment provider integration behind a feature boundary; Zarinpal remains intentionally deferred per the definition.

Delivered with six tenant-seeded revenue services, paid request totals, pre-arrival-only upsells, hotel-timezone availability windows, and pay-at-hotel checkout. Public and authenticated content APIs now cover facilities, promotions, restaurants, and menus with tenant-scoped administration and activation controls. The handoff-based React experience includes public hotel discovery, live guest paid ordering, real pre-arrival orders, and an administrator content workspace. Payment-provider capture remains deliberately outside the boundary.

## M5 — Conversations and AI knowledge (complete)

- Implement guest conversations, staff handoff, unread/read state, and message history.
- Add AI provider abstraction with confidence threshold and deterministic handoff to reception when confidence is low.
- Deliver knowledge item authoring, review, approve/reject workflow, and versioned publication.
- Add safety, prompt-injection, privacy, and retention controls before enabling production AI traffic.

Delivered with one stay-scoped conversation per guest, persisted guest/AI/staff/system messages, read cursors and derived unread counts, reception assignment/close flows, and tenant/stay-filtered live chat events. The default provider is deterministic and receives approved tenant knowledge only; low-confidence questions and prompt-injection markers produce the handoff state and exact handoff copy from the design. Direct identifier patterns are redacted before persistence, messages expire under a configurable retention window with a hard-delete purge command, and no guest text is sent to an external model. Versioned knowledge submissions retain prior publication until a new version is approved. The React guest bubbles, handoff state, reception inbox, and moderation cards follow the supplied high-fidelity handoff.

## M6 — Reporting, hardening, and deployment (complete)

- Add operational and revenue reporting based on request/reservation history.
- Add audit trails, rate limiting, security headers, backups, migrations in CI, and observability.
- Move file storage behind an object-storage interface; local disk remains the development adapter.
- Add production Nginx TLS configuration, deployment runbooks, and end-to-end smoke tests.

Delivered with tenant- and role-scoped operational/revenue reports, hotel-timezone date ranges, server-authoritative IRR totals, active-room and reception-handoff metrics, paginated audit administration, correlation IDs in responses/logs/audits, layered API and mutation rate limits, API and Nginx security headers, an additive reporting-hardening migration, repeatable migration rehearsal in CI, guarded PostgreSQL backup/restore scripts, TLS deployment guidance, and public plus authenticated smoke checks. The existing private document storage was already isolated behind `documents.Storage`; local volume storage remains the development/single-VPS adapter. The reporting and security UI extends the supplied handoff's RTL admin system rather than introducing a separate visual language.

## M7 — Platform operations, CLI, CI/CD, backup, and infrastructure (in progress)

- Consolidate operational commands into a tested `hotelmate` CLI with human-readable and JSON output, stable exit codes, configuration validation, safe dry runs, and secret-safe logging.
- Turn the existing CI workflow into a supply-chain-aware delivery pipeline that builds immutable OCI images once, publishes signed artifacts and SBOMs, deploys to staging, and promotes the same image digest to production through an approval gate.
- Automate encrypted, off-host PostgreSQL and private-upload backups with retention, integrity checks, failure alerts, restore rehearsals, and measured recovery objectives.
- Select and codify the hosting provider, environments, DNS/TLS, networking, secrets, storage, observability, security, capacity, and disaster-recovery architecture through versioned infrastructure as code and architecture decisions.
- Prove zero-guesswork deployment and recovery with staging acceptance, production smoke checks, rollback rehearsal, and an isolated restore drill.

Delivered in the repository with the unified Go operations CLI and JSON/exit-code contract; a native Go stateful acceptance suite; mandatory confirmation and signal-safe environment locks; digest-pinned production Compose; pinned, scan-gated, SBOM/provenance/signing release automation; staging-to-protected-production promotion; strict PostgreSQL/private-upload recovery manifests with encrypted restic transfer and scheduled retention; an automated isolated recovery drill with RPO/RTO evidence; deploy evidence and application rollback; Prometheus/Grafana/Alertmanager/Loki/Alloy monitoring with PostgreSQL query telemetry; and an Ansible host-hardening baseline.

Local deployment, authenticated smoke, full acceptance, a distinct-image rollback/redeploy, a coordinated PostgreSQL/private-upload recovery set, and an isolated drill have passed; the drill measured local RPO at 268 seconds and RTO at 4 seconds. An operator-supplied Ubuntu VPS has also been hardened through Ansible, rebooted, converged with no drift, and validated as HTTP staging: seven migrations, authenticated smoke, full stateful acceptance, private-document access, and a checksummed PostgreSQL/upload recovery set with 162 catalog entries passed. CI #26 passed all five gates for the exact deployed commit, and the release job in release #8 published, scanned, SBOMed, signed, attested, and verified both GHCR images; GitHub-hosted staging still lacks its SSH environment configuration, so production was correctly skipped. The [local evidence](evidence/M7_LOCAL_VALIDATION_20260823.md), [external staging evidence](evidence/M7_STAGING_VALIDATION_20260823.md), discovery, work packages, dependencies, and acceptance gates are linked from [M7 platform operations and infrastructure](MILESTONE_7_PLATFORM_OPERATIONS.md). M7 remains in progress until [ADR-0007](adr/0007-platform-operations-decisions.md) is approved and signed registry-based staging/production promotion, encrypted off-host durability, a scheduled provider-backed drill against approved objectives, DNS/TLS renewal, replacement-host rebuild, and routed alert exercises produce evidence.

## M8 — Online check-in 2.0 and arrival readiness (prototype-defined; implementation planned)

- Productize the supplied prototype's direct online-check-in entry and pre-arrival banner as a secure, resumable journey: reservation proof, contact and identity details, multi-image/PDF evidence, consent, versioned e-signature, companion and arrival details, correction, and staff review.
- Preserve confirmed-reservation to pre-arrival-stay conversion and the prototype's pre-arrival entitlement rule: paid services and eligible offers may be available before arrival, while operational room services remain blocked until physical check-in.
- Add signed invitation links and QR recovery, real three-step progress, autosave/resume, room-ready waiting, exception handling, feature-controlled rollout, and an arrivals workspace that shows readiness instead of only document status.
- Instrument invitation, completion, abandonment, review, rework, approval, room-ready, and front-desk handling time while preserving tenant isolation, data minimization, document retention, and explicit approval gates.

## M9 — Guest and staff experience redesign (prototype-defined; implementation planned)

- Turn the supplied prototype's four entry modes—public discovery, resident login, online check-in, and staff login—into one shared accessible design system with lifecycle-aware navigation and branded hotel theming.
- Productize the public hotel guide, facilities, restaurants/menus, activities, promotions, resident home, quick actions, request tracking, and guest/reception conversation surfaces with search, detail, structured ordering, active/history separation, and complete loading/error/offline recovery.
- Consolidate the prototype's dashboard, requests, conversations, notifications, reservations, stays, catalog, knowledge, promotions, analytics, users, and settings into role-specific staff workspaces with enforced server authorization, SLA triage, ownership, and responsive task views.
- Add Persian/English/Arabic locale architecture, installable web-app behavior, performance budgets, usability testing, and WCAG 2.2 AA verification while using `IRANYekanXFaNum, sans-serif` consistently.

## M10 — Lifecycle communication and personalization (prototype-defined; implementation planned)

- Productize the prototype's multilingual conversational journey: detect the guest's language independently from numeral format, extract multi-detail service requests, ask only for missing information, and transfer low-confidence or sensitive cases to reception with context.
- Add consent-aware booking-to-post-stay communication through provider adapters for in-app, web push, email, SMS, and WhatsApp, including pre-arrival automation, request-status updates, one-hour service reminders, templates, retries, fallbacks, opt-out handling, and delivery evidence.
- Unify automated and human conversations plus staff notifications in a role-aware inbox with ownership, SLA state, guest context, notification preferences, translation assistance, and escalation.
- Personalize shortcuts, content, and offers by stay stage and approved guest preferences without exposing sensitive profile data to unrelated staff roles.

## M11 — Commerce, payments, upsells, and digital checkout (prototype-defined; implementation planned)

- Productize the prototype's quote loop for paid services: guest or public lead expresses intent, the assistant gathers missing details, reception proposes cost/date/time, the guest adjusts and confirms or cancels, and staff fulfill against one auditable order state.
- Preserve the prototype's commercial boundaries: public users provide minimal contact data only at paid conversion, pre-arrival guests can buy eligible services, complimentary operational requests never become paid leads, and hotel-controlled promotions use frequency caps and eligibility rules.
- Add inventory- and schedule-aware upsells, modifiers, baskets, room upgrades, early arrival, late checkout, transparent taxes/fees, and trustworthy revenue/conversion measurement.
- Introduce a PCI-scoped payment-provider boundary for payment links, deposits/holds, idempotent webhooks, refunds, reconciliation, and folio-safe accounting, then deliver feature-controlled contactless checkout with bill review, receipt, issue escalation, feedback, and recovery.

## M12 — Hospitality integrations, automation, and journey intelligence (prototype-defined in part; implementation planned)

- Replace the supplied prototype's local reservation/stay store and static analytics with authoritative, privacy-safe event data and observable, replayable connector contracts for PMS/CRS, POS, CRM, channel managers, payment providers, and mobile-key systems.
- Coordinate reservation confirmation, pre-arrival stay creation, folio, room readiness, service tasks, paid-order quotes, notifications, checkout, and key release through idempotent sync and explicit human exception queues.
- Turn the prototype's operations, AI, VAS, satisfaction, and review cards into reconciled journey funnels, SLA dashboards, revenue attribution, segmentation, cohort reporting, and controlled experimentation with governed metric definitions.
- Manage hotel capabilities—online check-in, digital registration, pre-arrival ordering, promotions, reminders, contactless checkout, and automated messaging—as audited, dependency-aware rollout controls rather than independent UI switches.

The benchmark, current-state UX audit, detailed scope, sequencing, product measures, and acceptance gates for M8–M12 are defined in [Product improvement milestones](PRODUCT_IMPROVEMENT_MILESTONES.md).

## Definition of done for each milestone

Each milestone is complete when its API contract, database migration, role/tenant authorization, React flow, automated tests, and local Docker path are present. UI-only or schema-only work does not close a milestone.
