# HotelMate

HotelMate connects hotel guests with reception and hotel operations: pre-arrival check-in, service requests, upsells, facilities, restaurant content, and AI-assisted conversations.

The current delivery completes Milestones 0–6 and delivers the provider-neutral M7 platform foundation: infrastructure, tenant-aware identity/access, reservation and stay lifecycle, room state management, private online check-in documents, service operations and paid ordering, hotel content, live request/chat events, approved-knowledge guest conversations with reception handoff, server-calculated reporting, audit administration, production hardening, a unified operations CLI, immutable release automation, verified recovery sets, host configuration, and operational monitoring. The Persian RTL guest/staff/admin interface follows the supplied design handoff.

Milestone 7 is in progress. Its provider-neutral code and local deployment/recovery paths are implemented, and an operator-supplied Ubuntu VPS has been hardened and validated as external HTTP staging. Production completion still requires the owner/provider decisions, domain/DNS and trusted TLS, registry authorization, protected GitHub environments, signed staging/production promotion, an encrypted off-host repository, a scheduled provider-backed recovery drill against approved objectives, and tested alert routing recorded in [docs/MILESTONE_7_PLATFORM_OPERATIONS.md](docs/MILESTONE_7_PLATFORM_OPERATIONS.md) and [ADR-0007](docs/adr/0007-platform-operations-decisions.md).

The current M7 development release is deployed locally at <http://localhost:3000> and on the supplied VPS as staging. CLI preflight/apply, authenticated smoke, the native Go stateful acceptance suite, a distinct-image rollback/redeploy, coordinated PostgreSQL/private-upload recovery sets, and an isolated automated local recovery drill have passed. The external host also passed Ansible convergence, reboot persistence, seven migrations, authenticated smoke, stateful acceptance, and on-host recovery-set verification. CI #21 passed all five gates, and release #5 published signed, attested, digest-pinned GHCR images plus an immutable release bundle. The local drill measured RPO at 268 seconds and RTO at 4 seconds; these are implementation evidence, not approved production objectives. See the [local](docs/evidence/M7_LOCAL_VALIDATION_20260823.md) and [external staging](docs/evidence/M7_STAGING_VALIDATION_20260823.md) evidence records.

Product improvement Milestones 8–12 are planned for online check-in 2.0, guest/staff UX, lifecycle communication and personalization, commerce and digital checkout, and hospitality integrations with journey intelligence. The local-app audit and Duve/Canary/HiJiffy benchmark are documented in [docs/PRODUCT_IMPROVEMENT_MILESTONES.md](docs/PRODUCT_IMPROVEMENT_MILESTONES.md).

## Stack

- **API:** Go 1.25.13, `net/http`, GORM
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

Useful commands are available in the root `Makefile`. `make doctor`, `make migrate-status`, `make backup`, and the deployment targets are thin aliases for the `hotelmate` operations CLI. When `AUTO_MIGRATE=true`, the development API applies reviewed GORM schema steps through the `hotelmate_schema_migrations` ledger; staging/production disable startup migration and run it explicitly in the locked deployment workflow.

Release operations include `make migrate`, `make backup BACKUP_DIR=/absolute/private/path`, `make recovery-drill CONFIG_FILE=/absolute/protected/drill.env`, and `make smoke BASE_URL=https://hotel.example.com`. Every mutation requires explicit confirmation through the alias. The CLI and JSON contract are documented in [docs/OPERATIONS_CLI.md](docs/OPERATIONS_CLI.md); deployment/incident procedures are in [docs/OPERATIONS_RUNBOOK.md](docs/OPERATIONS_RUNBOOK.md), and restoration remains deliberately guarded in [docs/BACKUP_RESTORE.md](docs/BACKUP_RESTORE.md).

Online check-in files are stored in the private `uploads` volume, never under the web root. Run `make purge-documents` regularly; production scheduling is documented in the deployment guide.

Chat messages use the configured `CHAT_RETENTION` window. Schedule `make purge-messages` so expired message bodies are physically deleted. The default concierge provider is local and deterministic: it can return only approved tenant knowledge, and it transfers low-confidence or suspicious instructions to reception.

Realtime clients connect to `/api/v1/events` and send `hotelmate.events` plus the current JWT as WebSocket subprotocol values. The REST request list and persisted event history remain authoritative after a reconnect.

Paid service prices and request totals are stored as integer IRR amounts in the existing `priceCents` compatibility fields. The Persian UI presents them as toman and uses pay-at-hotel checkout; no payment-provider credentials or capture flow are enabled.

## Guides

- [Beginner guide](docs/BEGINNER_GUIDE.md)
- [Production deployment](docs/DEPLOYMENT_GUIDE.md)
- [Backup and restore runbook](docs/BACKUP_RESTORE.md)
- [Operations CLI contract](docs/OPERATIONS_CLI.md)
- [Platform operations runbook](docs/OPERATIONS_RUNBOOK.md)
- [M7 platform operations and infrastructure](docs/MILESTONE_7_PLATFORM_OPERATIONS.md)
- [Product improvement milestones and benchmark](docs/PRODUCT_IMPROVEMENT_MILESTONES.md)
- [Foreign VPS notes](docs/FOREIGN_VPS_GUIDE.md)
