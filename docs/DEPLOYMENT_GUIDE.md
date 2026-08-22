# HotelMate production deployment guide

This guide prepares a single Linux VPS deployment with PostgreSQL, the Go API, the React build, Nginx, and an existing Let's Encrypt certificate. A real public deployment additionally needs a VPS, DNS name, SSH access, and certificate; those external resources are not stored in this repository.

## 1. Server prerequisites

- Ubuntu 24.04 or a comparable maintained Linux distribution
- Docker Engine with the Compose plugin
- A DNS `A`/`AAAA` record pointing the chosen domain to the server
- Ports 22, 80, and 443 allowed in the firewall
- A Let's Encrypt certificate under `/etc/letsencrypt/live/<domain>`

Obtain the initial certificate before starting the TLS profile, for example with Certbot standalone while port 80 is free. Certificate issuance and renewal are server administration operations and should be automated by the operator.

## 2. Configure secrets

```bash
cp .env.production.example .env.production
chmod 600 .env.production
```

Replace every placeholder. Generate independent random values for the database password, `JWT_SECRET`, and `ONBOARDING_TOKEN`. Because the database password is embedded in a PostgreSQL URL by Compose, use URL-safe characters or percent-encode reserved characters. Set `DOMAIN` and `ALLOWED_ORIGINS=https://<domain>`.

The API rejects placeholder or short production secrets at startup.

## 3. Deploy

```bash
docker compose --env-file .env.production -f docker-compose.production.yml config --quiet
docker compose --env-file .env.production -f docker-compose.production.yml build
docker compose --env-file .env.production -f docker-compose.production.yml up -d
docker compose --env-file .env.production -f docker-compose.production.yml ps
```

Verify:

```bash
curl -fsS https://<domain>/healthz
curl -fsS https://<domain>/readyz
./scripts/smoke.sh https://<domain>
```

## 4. Create the first hotel

Call `POST /api/v1/onboarding/hotels` once with the `X-Onboarding-Token` header. The request schema and response are defined in `docs/openapi.yaml`. Keep this token out of browser code and rotate it after initial onboarding when operational policy requires it.

## 5. Updates and rollback

Before an update, take a PostgreSQL backup and retain the currently deployed Git commit/image. Then pull the reviewed commit, build, and run `up -d`. The application records applied schema versions in `hotelmate_schema_migrations`.

M1 through M6 use additive migrations. M3 adds and backfills tenant-safe service codes, derives `service_requests.hotel_id` from each stay, converts the legacy assignment column to UUID, creates persisted request events, and seeds missing core services per hotel. M4 adds paid/pre-arrival service metadata and hotel content fields, then seeds missing revenue services and starter content per hotel. M5 adds conversation read/assignment/retention state plus versioned knowledge moderation and seeds six approved starter topics for hotels without knowledge. M6 adds request correlation to the audit schema through `2026082206_reporting_hardening`. Run `/app/migrate` twice in staging before deployment to rehearse and prove idempotence. An M0 development volume may still contain the obsolete plaintext `guests.identity_number` column and global staff email constraint. Do not remove either automatically on a database that may contain data; inspect and migrate that volume explicitly first.

## 6. WebSocket delivery

Nginx proxies `/api/v1/events` through the existing `/api/` location with HTTP/1.1 upgrade headers and a read timeout longer than the API heartbeat. Browser clients connect with `wss://<domain>/api/v1/events` and provide `hotelmate.events` plus the current JWT as WebSocket subprotocol values. Keep `ALLOWED_ORIGINS` aligned with the public HTTPS origin.

Persisted request history from the REST API is authoritative after reconnecting. The bundled realtime hub is process-local, so deploy one API replica for M3. Add a shared pub/sub transport before scaling the API horizontally.

## 7. Private document retention

Online check-in documents are written to the private `uploads-production` volume and are never served by Nginx. Keep `DOCUMENT_MAX_BYTES` and `DOCUMENT_RETENTION` explicit in `.env.production`; the default retention is 720 hours after the later of submission or scheduled departure.

Schedule the purge command at least daily from the deployment directory. For example, an operator-owned cron entry can run:

```cron
17 3 * * * cd /srv/HotelMate && docker compose --env-file .env.production -f docker-compose.production.yml run --rm --entrypoint /app/purge-documents api >> /var/log/hotelmate-document-purge.log 2>&1
```

Run `make purge-documents` for the local Compose stack. The command deletes only documents whose stored retention deadline has passed and then marks their metadata deleted. Monitor failures; do not expose or manually sweep the volume as a substitute for this workflow.

## 8. Backups and restore rehearsal

Run `./scripts/backup.sh /absolute/private/backup/path` from the deployment directory. It writes a PostgreSQL custom-format dump with mode-restrictive defaults plus a SHA-256 checksum. Store the output on encrypted storage outside the Docker host/volume and schedule it with the operator's job runner. The private document adapter remains backed by the `uploads-production` volume on a single VPS; if policy requires document recovery, separately encrypt and access-control that volume backup and expire copies in line with document retention.

Restoration replaces matching database objects and therefore requires the explicit `--confirm` argument. Stop the API, verify the target environment and checksum, run the guarded restore script, start the API, and execute the smoke checks. The exact drill is in `docs/BACKUP_RESTORE.md`. Test it in an isolated environment at least quarterly and before a major schema rollout.

## 9. Conversation privacy and retention

Set `CHAT_RETENTION` to the approved privacy period and `CHAT_CONFIDENCE_THRESHOLD` between 0 and 1. The default is 90 days and 0.5. Schedule `/app/purge-messages` at least daily, using the same Compose pattern as the document purge. This command performs a hard delete of expired message bodies; monitor its exit status. Keep the bundled deterministic provider unless an external provider has completed a separate data-protection, prompt-safety, and credential review.

## 10. Security and observability

Keep `ENABLE_HSTS=true` only behind working HTTPS. Nginx and the API emit the release security headers. All API responses include `X-Request-ID`; use it to correlate browser errors, structured API logs, and administrator audit entries. The in-memory API limits are a single-replica safeguard. Add an edge/shared rate limiter before horizontal scaling. Alert on readiness failures, repeated HTTP 5xx responses, failed purge/backup jobs, and increases in failed security events.
