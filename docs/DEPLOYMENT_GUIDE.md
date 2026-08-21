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
```

## 4. Create the first hotel

Call `POST /api/v1/onboarding/hotels` once with the `X-Onboarding-Token` header. The request schema and response are defined in `docs/openapi.yaml`. Keep this token out of browser code and rotate it after initial onboarding when operational policy requires it.

## 5. Updates and rollback

Before an update, take a PostgreSQL backup and retain the currently deployed Git commit/image. Then pull the reviewed commit, build, and run `up -d`. The application records applied schema versions in `hotelmate_schema_migrations`.

M1 and M2 use additive migrations. An M0 development volume may still contain the obsolete plaintext `guests.identity_number` column and global staff email constraint. Do not remove either automatically on a database that may contain data; inspect and migrate that volume explicitly first.

## 6. Private document retention

Online check-in documents are written to the private `uploads-production` volume and are never served by Nginx. Keep `DOCUMENT_MAX_BYTES` and `DOCUMENT_RETENTION` explicit in `.env.production`; the default retention is 720 hours after the later of submission or scheduled departure.

Schedule the purge command at least daily from the deployment directory. For example, an operator-owned cron entry can run:

```cron
17 3 * * * cd /srv/HotelMate && docker compose --env-file .env.production -f docker-compose.production.yml run --rm --entrypoint /app/purge-documents api >> /var/log/hotelmate-document-purge.log 2>&1
```

Run `make purge-documents` for the local Compose stack. The command deletes only documents whose stored retention deadline has passed and then marks their metadata deleted. Monitor failures; do not expose or manually sweep the volume as a substitute for this workflow.

## 7. Backups

At minimum, schedule encrypted `pg_dump` backups outside the Docker volume and test restoration regularly. Upload files remain on a Docker volume until the object-storage milestone is delivered. If policy requires those files to be recoverable, encrypt and access-control the volume backup and expire backup copies in line with the document retention policy.
