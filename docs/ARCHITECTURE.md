# HotelMate architecture

## Stack decision

The attached `PROJECT_DEFINITION.md` describes the product and originally lists Node.js, Express, Prisma, and plain JavaScript. The explicit development request overrides that implementation table:

- **Backend:** Go 1.24, standard `net/http`, and GORM.
- **Frontend:** React + TypeScript, built with Vite.
- **Database:** PostgreSQL 16.
- **Local orchestration:** Docker Compose with Postgres, API, and an Nginx-served web build.

The product workflows, roles, and domain entities remain those in the definition. The repository now contains the infrastructure baseline plus the complete M1 identity/access and M2 reservation/stay lifecycle slices; later business modules are delivered milestone by milestone.

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

Real-time transport (WebSocket gateway) and external providers are deliberately introduced in later milestones rather than coupling the identity slice to unfinished product behavior.
