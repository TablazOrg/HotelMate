# HotelMate architecture

## Stack decision

The attached `PROJECT_DEFINITION.md` describes the product and originally lists Node.js, Express, Prisma, and plain JavaScript. The explicit development request overrides that implementation table:

- **Backend:** Go 1.24, standard `net/http`, and GORM.
- **Frontend:** React + TypeScript, built with Vite.
- **Database:** PostgreSQL 16.
- **Local orchestration:** Docker Compose with Postgres, API, and an Nginx-served web build.

The product workflows, roles, and domain entities remain those in the definition. This repository is intentionally starting with infrastructure and a minimal API/UI health slice; business modules are delivered milestone by milestone.

## Repository layout

```text
backend/
  cmd/api/              process entrypoint and graceful shutdown
  internal/config/      environment-backed configuration
  internal/database/    GORM connection and migrations
  internal/httpapi/     HTTP handlers and middleware
  internal/models/      domain model foundations
frontend/
  src/                  React application shell (RTL/Farsi first)
docs/
  ARCHITECTURE.md
  MILESTONES.md
```

## Runtime boundaries

The API owns authentication, authorization, hotel operations, catalog data, conversations, and persistence. The React app consumes versioned `/api/v1` endpoints. PostgreSQL is the system of record. Real-time transport (WebSocket/Socket.IO-compatible gateway) and external providers are deliberately introduced in later milestones rather than coupling the initial scaffold to unfinished product behavior.
