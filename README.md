# HotelMate

HotelMate connects hotel guests with reception and hotel operations: pre-arrival check-in, service requests, upsells, facilities, restaurant content, and AI-assisted conversations.

The current delivery includes Milestones 0–2: infrastructure, tenant-aware identity/access, reservation and stay lifecycle, room state management, pre-arrival guest login, and private online check-in documents. Reception can create and confirm reservations, check guests in and out, and review documents from the responsive RTL operations UI. Later product modules remain tracked in the milestone plan.

## Stack

- **API:** Go 1.24, `net/http`, GORM
- **Web:** React + TypeScript + Vite
- **Database:** PostgreSQL 16
- **Local runtime:** Docker Compose + Nginx

The attached project definition is the product source of truth. Its original Node/Prisma implementation table is superseded by the stack above, as requested for this development phase. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/MILESTONES.md](docs/MILESTONES.md), and [docs/openapi.yaml](docs/openapi.yaml).

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

Create a local hotel, primary administrator, room, guest, and active stay with:

```bash
make seed-demo
```

The seed is disabled in production and prints its local demo credentials.

## Local development without Docker

1. Start PostgreSQL and set `DATABASE_URL` (see `.env.example`).
2. Run the API: `cd backend && go run ./cmd/api`.
3. Install and run the web app: `cd frontend && npm install && npm run dev`.

Useful commands are available in the root `Makefile`. When `AUTO_MIGRATE=true`, the API applies reviewed GORM schema steps through the `hotelmate_schema_migrations` ledger.

Online check-in files are stored in the private `uploads` volume, never under the web root. Run `make purge-documents` regularly; production scheduling is documented in the deployment guide.

## Guides

- [Beginner guide](docs/BEGINNER_GUIDE.md)
- [Production deployment](docs/DEPLOYMENT_GUIDE.md)
- [Foreign VPS notes](docs/FOREIGN_VPS_GUIDE.md)
