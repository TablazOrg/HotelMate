# HotelMate staging and production deployment

The M7 release path deploys immutable API/web image digests to a hardened single-host Compose environment. It does not build application images on a server. The initial production values and two temporary control exceptions are accepted in [ADR-0007](adr/0007-platform-operations-decisions.md); provider-account metadata, SSH keys, and secrets remain outside the repository.

## 1. Approve and provision external resources

For a new environment, approve provider/region/residency/budget, domains/DNS/TLS ownership, RPO/RTO/retention/document recovery, environments/approvers, registry/secrets/restic destination, monitoring/paging owners, and access/break-glass policy. The initial `hotelmate.ir` environment uses the ADR-0007 values and records off-host storage and external alert delivery as temporary exceptions.

Create staging and production Ubuntu 24.04 hosts and DNS records. Obtain the first ACME certificates under `/etc/letsencrypt/live/<domain>` and configure automated renewal. Only approved SSH, HTTP, and HTTPS may be reachable; PostgreSQL binds to host loopback and the Compose network.

## 2. Apply the versioned host baseline

Install the listed Ansible collections, create real ignored `group_vars/all.yml` and Vault-encrypted `group_vars/all/vault.yml`, supply the release-built CLI path, and run:

```bash
cd infra/ansible
ansible-galaxy collection install -r requirements.yml
ansible-playbook -i inventory.yml playbook.yml --ask-vault-pass \
  -e hotelmate_cli_artifact=/absolute/path/hotelmate-linux-amd64 \
  -e hotelmate_cli_sha256=<trusted-release-sha256> \
  -e hotelmate_cosign_artifact=/absolute/path/cosign-linux-amd64 \
  -e hotelmate_cosign_sha256=<trusted-cosign-sha256>
```

The playbook verifies both controller-side artifacts before transfer, then creates the non-root deploy user, key-only SSH, default-deny firewall, unattended security updates, Docker/PostgreSQL/restic/Buildx tooling, optional managed swap, Docker JSON-log rotation, Certbot renewal and its fail-closed Nginx reload hook, protected runtime directories/config, and systemd backup/privacy-retention timers. It resolves the operator GID and shares private uploads only between that group and the unprivileged API UID; `HOTELMATE_API_UID` and `HOTELMATE_OPERATOR_GID` are rendered into the protected environment. The optional monthly recovery-drill timer remains disabled until a dedicated protected non-production environment is supplied. Production refuses an unapproved alert receiver unless the explicit ADR-0007 deferral flag and reason are supplied.

After the first certificate is issued, configure Certbot to use the Compose webroot rather than retaining the standalone authenticator, then prove renewal and the installed deploy hook:

```bash
sudo certbot reconfigure --cert-name <domain> --webroot \
  --webroot-path /var/lib/docker/volumes/hotelmate_certbot-webroot/_data \
  --non-interactive
sudo certbot renew --cert-name <domain> --dry-run --run-deploy-hooks \
  --non-interactive
```

The renewal hook validates the running edge configuration and reloads exactly one `hotelmate` Compose web container. It fails closed if that invariant is not met.

Copy reviewed runtime files to `/srv/hotelmate`, including production and observability Compose files, `ops/observability`, and the frontend Nginx template as packaged by the release process. `docker-compose.production.yml` requires `HOTELMATE_API_IMAGE`, `HOTELMATE_WEB_IMAGE`, and `HOTELMATE_OPERATOR_GID`; the CLI writes release image digests from the manifest into a mode-`600` managed file. `HOTELMATE_PULL_POLICY` defaults to `always`; `never` is reserved for an explicitly documented non-production controller-loaded image exercise and is not a production signature bypass.

## 3. Configure GitHub environments

Create `staging` and `production` environments. Require the approved production reviewers and prevent self-review where policy requires it. Configure:

- Environment secrets: `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY`, pinned `DEPLOY_KNOWN_HOSTS`, all three authenticated-smoke values, and the staging-only acceptance onboarding token.
- Environment variables: `DEPLOY_PATH`, `CONFIG_FILE`, and `BASE_URL`.
- Package permission for GHCR and branch protection requiring every CI job.

The main-branch release workflow builds each image once, publishes by commit tag, scans it, emits maximum provenance and SPDX SBOMs, signs image/SBOM attestations, verifies them, and creates `hotelmate.release/v1`. It deploys staging automatically after CI and reaches production only through the protected environment. A manual emergency dispatch follows the same gates.

## 4. Manual preflight and deployment

The workflow normally owns promotion. An authorized manual run uses the exact release bundle:

```bash
hotelmate --config /etc/hotelmate/production.env \
  --release-file /srv/hotelmate/incoming/release.json deploy preflight

hotelmate --config /etc/hotelmate/production.env \
  --release-file /srv/hotelmate/incoming/release.json deploy apply --yes

hotelmate --config /etc/hotelmate/production.env deploy status --json
```

Preflight validates secret-safe application config, protected file modes, digest references, registry access, cosign image signatures and SPDX attestations, Compose rendering, off-host backup configuration, and tooling. Apply takes the environment lock, creates/verifies/transfers the production checkpoint, pulls digests, starts PostgreSQL, runs `hotelmate migrate up`, activates API/web, runs smoke, and records timestamped evidence. Failed activation/smoke attempts application rollback to the prior manifest.

Run the stateful acceptance suite only in staging because it creates dedicated tenants:

```bash
ACCEPTANCE_ONBOARDING_TOKEN='<staging-only-token>' \
  hotelmate --base-url https://staging.example.com acceptance
```

Production runs public and dedicated-account authenticated smoke. The acceptance suite is implemented in Go and covers onboarding defaults, RBAC/tenant isolation, reservation/stay lifecycle, paid ordering, private documents, AI handoff, fulfillment, reporting/audits, checkout, and expired-session rejection; `scripts/acceptance.sh` is only a compatibility launcher.

## 5. Rollback

Select the last known-good manifest from evidence, confirm its compatibility with applied additive migrations, then run:

```bash
hotelmate --config /etc/hotelmate/production.env \
  --release-file /srv/hotelmate/releases/known-good.json deploy rollback --yes
```

This changes API/web images and verifies smoke. It does not reverse database migrations. Use a reviewed forward fix or [recovery procedure](BACKUP_RESTORE.md) when schema/data repair is required.

## 6. Monitoring, schedules, and retention

Start production plus the private operations profile after replacing the external probe placeholder and configuring the approved Alertmanager receiver:

```bash
docker compose --env-file /etc/hotelmate/production.env \
  -f docker-compose.production.yml -f docker-compose.observability.yml up -d
```

Prometheus, Grafana, and Alertmanager bind to loopback for SSH tunnel/VPN access. Validate every rule in staging. The API remains one replica because realtime fanout is process-local.

Systemd runs a daily recovery set and daily document/message purges. When explicitly approved, it also runs the monthly isolated recovery drill. Confirm with `systemctl list-timers 'hotelmate-*'`, inspect each unit result, and verify last-run/last-success/failure metrics plus backup and drill ages. Document/message retention and drill objectives remain explicit in protected environments.

## 7. Operational acceptance

Before calling M7 complete, retain evidence of clean Ansible re-provisioning, no unexpected infrastructure/config drift, same digest in staging/production, restore within approved RPO/RTO, deploy and failed-deploy rollback, DNS/TLS renewal, firewall/access review, patching, disk capacity, central logs/metrics retention, every alert route, and the incident exercises in [OPERATIONS_RUNBOOK.md](OPERATIONS_RUNBOOK.md).
