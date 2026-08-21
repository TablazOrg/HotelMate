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

## M4 — Revenue and hotel content

- Implement paid services and pre-arrival upsell flows with prices/currency and order totals.
- Add facilities, promotions, restaurants, and menu management plus guest browsing views.
- Add availability windows, activation controls, and tenant-scoped content administration.
- Keep payment provider integration behind a feature boundary; Zarinpal remains intentionally deferred per the definition.

## M5 — Conversations and AI knowledge

- Implement guest conversations, staff handoff, unread/read state, and message history.
- Add AI provider abstraction with confidence threshold and deterministic handoff to reception when confidence is low.
- Deliver knowledge item authoring, review, approve/reject workflow, and versioned publication.
- Add safety, prompt-injection, privacy, and retention controls before enabling production AI traffic.

## M6 — Reporting, hardening, and deployment

- Add operational and revenue reporting based on request/reservation history.
- Add audit trails, rate limiting, security headers, backups, migrations in CI, and observability.
- Move file storage behind an object-storage interface; local disk remains the development adapter.
- Add production Nginx TLS configuration, deployment runbooks, and end-to-end smoke tests.

## Definition of done for each milestone

Each milestone is complete when its API contract, database migration, role/tenant authorization, React flow, automated tests, and local Docker path are present. UI-only or schema-only work does not close a milestone.
