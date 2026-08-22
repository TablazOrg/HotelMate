# HotelMate beginner guide

## What is currently available

Milestones 0–6 are implemented: the Go/PostgreSQL/React runtime, hotel onboarding, separate staff and guest JWT sessions, tenant and role enforcement, public hotel branding and content, staff account management, reservation/check-in/check-out operations, pre-arrival login and document upload, free and paid service requests, operations queues, realtime updates, approved-knowledge conversations with reception handoff, operational/revenue reporting, security audit administration, backups, and production hardening in the handoff-based RTL UI.

## Start the project

1. Install Docker Desktop (or another Docker + Compose runtime).
2. Copy `.env.example` to `.env`.
3. Change `JWT_SECRET` and `ONBOARDING_TOKEN` if the machine is shared.
4. Run `docker compose up --build`.
5. Open <http://localhost:3000>.

The API health endpoints are available through the web proxy at <http://localhost:3000/healthz> and <http://localhost:3000/readyz>.

## Create a local demo

Run:

```bash
make seed-demo
```

The command is disabled when `APP_ENV=production`. It prints local-only staff and guest credentials after creating an active demo stay. Override all demo values with the `DEMO_*` environment variables accepted by `backend/cmd/seed-demo`.

## Useful commands

```bash
make backend-test
make frontend-build
make purge-messages
make migrate
make smoke BASE_URL=http://localhost:3000
make logs
make down
```

The API contract is in `docs/openapi.yaml`; product sequencing is in `docs/MILESTONES.md`.
