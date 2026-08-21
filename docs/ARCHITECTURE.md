# HotelMate architecture

## Stack decision

The attached `PROJECT_DEFINITION.md` describes the product and originally lists Node.js, Express, Prisma, and plain JavaScript. The explicit development request overrides that implementation table:

- **Backend:** Go 1.24, standard `net/http`, and GORM.
- **Frontend:** React + TypeScript, built with Vite.
- **Database:** PostgreSQL 16.
- **Local orchestration:** Docker Compose with Postgres, API, and an Nginx-served web build.

The product workflows, roles, and domain entities remain those in the definition. The repository now contains the infrastructure baseline plus the complete M1 identity/access slice; later business modules are delivered milestone by milestone.

## Repository layout

```text
backend/
  cmd/api/              process entrypoint and graceful shutdown
  internal/config/      environment-backed configuration
  internal/database/    GORM connection and migrations
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

The API owns authentication, authorization, hotel operations, catalog data, conversations, and persistence. The React app consumes versioned `/api/v1` endpoints. PostgreSQL is the system of record. Staff and guest tokens use distinct JWT audiences and every authenticated lookup is constrained by the signed hotel ID; active guest sessions are additionally constrained by stay ID. Identity numbers are compared only against bcrypt hashes and are excluded from responses and audit metadata.

Real-time transport (WebSocket gateway) and external providers are deliberately introduced in later milestones rather than coupling the identity slice to unfinished product behavior.
