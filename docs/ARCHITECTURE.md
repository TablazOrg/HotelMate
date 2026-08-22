# HotelMate architecture

## Stack decision

The attached `PROJECT_DEFINITION.md` describes the product and originally lists Node.js, Express, Prisma, and plain JavaScript. The explicit development request overrides that implementation table:

- **Backend:** Go 1.24, standard `net/http`, and GORM.
- **Frontend:** React + TypeScript, built with Vite.
- **Database:** PostgreSQL 16.
- **Local orchestration:** Docker Compose with Postgres, API, and an Nginx-served web build.

The product workflows, roles, and domain entities remain those in the definition. The repository contains complete M0–M6 slices: infrastructure, identity/access, reservation/stay lifecycle, service operations and realtime delivery, paid/pre-arrival services, hotel content, approved-knowledge conversations, operational/revenue reporting, audit administration, deployment hardening, and the approved handoff-driven React flows.

## Repository layout

```text
backend/
  cmd/api/              process entrypoint and graceful shutdown
  internal/config/      environment-backed configuration
  internal/database/    GORM connection and migrations
  internal/documents/   private local document-storage adapter
  internal/auth/        password/identity hashing and audience-bound JWTs
  internal/httpapi/     HTTP handlers and middleware
  internal/models/      domain model foundations
  internal/realtime/    tenant/stay/department-scoped event fanout
  internal/store/       tenant-scoped GORM persistence
scripts/                guarded backup, restore, and release smoke checks
frontend/
  src/                  React application shell (RTL/Farsi first)
docs/
  ARCHITECTURE.md
  MILESTONES.md
```

## Runtime boundaries

The API owns authentication, authorization, hotel operations, catalog data, conversations, and persistence. The React app consumes versioned `/api/v1` endpoints. PostgreSQL is the system of record. Staff and guest tokens use distinct JWT audiences and every authenticated lookup is constrained by the signed hotel ID; guest sessions are additionally constrained by stay ID and allow only pre-arrival or active stays. Identity numbers are compared only against bcrypt hashes and are excluded from responses and audit metadata.

Reservation confirmation creates exactly one pre-arrival stay in the same transaction. Check-in requires the assigned room to be available and marks it occupied; checkout completes the reservation and marks the room cleaning. Reception, operations managers, and hotel administrators may perform these transitions.

Online check-in document bytes live outside the web root behind the `documents.Storage` interface. The local adapter generates server-side keys, MIME-sniffs PDF/JPEG/PNG content, enforces the configured size limit, stores files with restrictive permissions, and exposes bytes only through an authenticated, tenant-scoped staff route. PostgreSQL stores metadata and the retention deadline; the purge command performs explicit expiry deletion.

Service requests are hotel- and stay-scoped, snapshot their calculated total, and append immutable creation, assignment, priority, status, and note events. The state machine permits `new → in_progress → completed`, with cancellation from open states. Housekeeping and F&B queues are forcibly constrained to their own categories; reception, operations, and administrators can see the hotel-wide queue.

Paid services reuse the same request state machine and snapshot quantity × price in integer IRR. Active stays may order active services; pre-arrival stays are restricted to paid services explicitly marked for pre-arrival. Optional daily availability windows are evaluated in the hotel's configured IANA timezone and support overnight windows. Payment capture is behind a product boundary: the current UI records pay-at-hotel orders only.

Facilities, promotions, restaurants, and menu items are hotel-owned content. Public reads return only currently active/available records, while authenticated staff reads include inactive records for editing. All writes derive the hotel from the staff JWT and are restricted to administrators and operations managers.

The `/api/v1/events` WebSocket gateway authenticates the existing JWT through the negotiated subprotocol list. The in-process hub filters publications by hotel, guest stay, and staff department. Slow subscribers are skipped rather than blocking operations; clients recover from persisted REST history after reconnecting. The current hub supports a single API replica. Horizontal API scaling requires a shared pub/sub adapter before multiple replicas are enabled.

The concierge provider is an interface boundary. The shipped provider performs deterministic matching over the latest approved knowledge version for the signed hotel; drafts and rejected answers never enter its context. Confidence below `CHAT_CONFIDENCE_THRESHOLD` and recognized prompt-injection markers bypass answer generation and permanently move the conversation to reception handoff. Common email and long-number identifiers are redacted before storage. Message reads exclude expired content, and `purge-messages` hard-deletes expired rows under `CHAT_RETENTION`.

Operational reports are calculated in the API from tenant-scoped request history, completion timestamps, snapshotted IRR totals, active stays, conversations, knowledge moderation state, and failed audit events. Report ranges are bounded to 31 hotel-local calendar days. The React reporting and security views reuse the supplied handoff's admin shell, card geometry, spacing, typography, RTL direction, and tenant color tokens; the browser never derives authoritative revenue totals.

Every HTTP request receives a validated or generated `X-Request-ID`. The same value is returned to the client, attached to structured request logs, and persisted on mutation/security audit records. Layered in-process limits cover all API traffic, mutation traffic, authentication, and onboarding. The API and Nginx add CSP, Permissions Policy, framing, MIME-sniffing, referrer, cross-origin resource, and production HSTS controls. Health and readiness probes, request status/duration logging, and the audit/reporting views provide the single-replica observability baseline.

No external AI or payment provider is enabled by default.
