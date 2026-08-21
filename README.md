# HotelMate

HotelMate connects hotel guests with reception and hotel operations: pre-arrival check-in, service requests, upsells, facilities, restaurant content, and AI-assisted conversations.

## Stack

- **API:** Go 1.24, `net/http`, GORM
- **Web:** React + TypeScript + Vite
- **Database:** PostgreSQL 16
- **Local runtime:** Docker Compose + Nginx

The attached project definition is the product source of truth. Its original Node/Prisma implementation table is superseded by the stack above, as requested for this development phase. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/MILESTONES.md](docs/MILESTONES.md).

## Quick start

```bash
cp .env.example .env
docker compose up --build
```

Then open:

- Web app: <http://localhost:3000>
- API health: <http://localhost:8080/healthz>
- API readiness: <http://localhost:8080/readyz>

If port `8080` is already in use, set `API_PORT=8081` and
`VITE_API_BASE_URL=http://localhost:8081` in `.env` before starting Compose.

## Local development without Docker

1. Start PostgreSQL and set `DATABASE_URL` (see `.env.example`).
2. Run the API: `cd backend && go run ./cmd/api`.
3. Install and run the web app: `cd frontend && npm install && npm run dev`.

Useful commands are available in the root `Makefile`. The initial API runs GORM `AutoMigrate` in development; production deployments should use reviewed, versioned migrations before rollout.
