# HotelMate architecture

## Stack decision

The attached `PROJECT_DEFINITION.md` describes the product and originally lists Node.js, Express, Prisma, and plain JavaScript. The explicit development request overrides that implementation table:

- **Backend:** Go 1.24, standard `net/http`, and GORM.
- **Frontend:** React + TypeScript, built with Vite.
- **Database:** PostgreSQL 16.
- **Local orchestration:** Docker Compose with Postgres, API, and an Nginx-served web build.

The product workflows, roles, and domain entities remain those in the definition. The repository now contains the infrastructure baseline, complete M1 identity/access and M2 reservation/stay lifecycle slices, and the M3 service-operations backend/realtime foundation. M3 React screens are gated on the approved design handoff and are not finalized from inferred styling.

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

The `/api/v1/events` WebSocket gateway authenticates the existing JWT through the negotiated subprotocol list. The in-process hub filters publications by hotel, guest stay, and staff department. Slow subscribers are skipped rather than blocking operations; clients recover from persisted REST history after reconnecting. The current hub supports a single API replica. Horizontal API scaling requires a shared pub/sub adapter before multiple replicas are enabled.

External providers remain behind later milestone boundaries.
