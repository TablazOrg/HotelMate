# HotelMate platform operations runbook

This runbook implements the accepted choices in [ADR-0007](adr/0007-platform-operations-decisions.md). Its off-host recovery and external paging steps remain unavailable under the two recorded owner-approved exceptions; do not represent either control as active until its protected credentials and exercise evidence exist.

## Deploy and promote

1. Confirm CI passed and the release job verified image signatures, vulnerability policy, provenance, and both SPDX attestations.
2. Review the `hotelmate.release/v1` manifest. Staging and production image digests must be identical.
3. Run `hotelmate --config <protected-env> --release-file <manifest> deploy preflight`.
4. Run `deploy apply --yes`. The CLI locks the environment, takes and transfers the production recovery checkpoint, starts PostgreSQL, runs migrations, activates the digest-pinned images, and runs smoke checks.
5. Run the stateful acceptance suite in staging. Production receives public and authenticated smoke checks only.
6. Retain the evidence JSON and link it from the change record.

The GitHub workflow performs these steps automatically. The `production` environment must require the approved reviewers. An emergency dispatch uses the same build, scan, signing, backup, lock, and verification path.

## Roll back an application release

1. Identify the last known-good release manifest from deployment evidence.
2. Review whether intervening migrations remain backward-compatible. If not, stop and approve a forward-fix or isolated restore plan.
3. Run `hotelmate --config <protected-env> --release-file <known-good-manifest> deploy rollback --yes`.
4. Confirm public/authenticated smoke, readiness, 5xx rate, and WebSocket health.
5. Record the incident/change link. Database state is not rolled back by this command.

Failed activation or smoke automatically attempts this application-only rollback when a previous manifest exists.

## Restore and disaster recovery

Follow [BACKUP_RESTORE.md](BACKUP_RESTORE.md). Always restore a drill into an isolated database, private-upload path, and application target. `hotelmate backup drill --yes` refuses production and requires the explicit isolated-drill flag; it records source/requested timestamps, release, schema ledger, checksum/catalog verification, restore duration, actual RPO/RTO, authenticated smoke, tenant-isolation acceptance, and operator. Enable the monthly timer only after its dedicated protected environment and objectives are approved.

For complete host loss, provision the approved provider resource, run the Ansible playbook, restore the protected config from the secrets manager, initialize TLS/DNS, restore the selected recovery set, deploy its matching release manifest, and run smoke plus acceptance before moving traffic.

For database corruption or accidental deletion, freeze writes, preserve forensic state, identify the last verified recovery set, and choose restore versus forward repair with the incident commander. Never overwrite the only surviving copy.

## Certificate, credential, and image incidents

- Certificate expiry: remove the host from traffic if TLS is invalid, renew using the approved ACME owner, verify the full chain and external probe, then restore traffic. The normal path is `certbot renew`; the installed deploy hook validates and reloads the running edge. Exercise it safely with `certbot renew --dry-run --run-deploy-hooks`. Do not disable HTTPS/HSTS as a workaround.
- Compromised deploy/SSH credential: revoke it, remove authorized access, rotate dependent secrets, review audit/GitHub/SSH logs, rebuild the host if integrity is uncertain, and complete the access review.
- Compromised application secret: rotate JWT/onboarding/database/restic credentials in the secrets manager and environment, invalidate active sessions where applicable, deploy, and verify.
- Compromised image: block the digest in the registry, roll back to a verified manifest, preserve artifacts for investigation, rebuild from a reviewed commit, reissue SBOM/provenance/signature, and promote normally.

## Monitoring and alerts

The private observability profile supplies Prometheus, Grafana, Alertmanager, Loki with Alloy Docker-log collection, node/container/PostgreSQL exporters, an external TLS probe, a provisioned platform dashboard, and rules for availability, readiness, 5xx rate, latency, authentication failures, WebSocket failures, release identity, operation failures, certificate expiry, backup/purge/drill freshness, CPU/memory/disk/inodes/private-volume capacity, restarts, PostgreSQL connections/query latency/storage, and log-pipeline availability. `pg_stat_statements` and I/O timing support query telemetry. Edge and API logs retain request ID as structured metadata rather than a high-cardinality label. Prometheus/Grafana/Alertmanager bind to loopback only; Loki is reachable only on the Compose network.

Before enabling an environment:

1. Replace the example external target with the approved domain.
2. Replace the no-op Alertmanager receiver with the approved paging destination and escalation route.
3. Trigger every page/ticket rule in staging and retain evidence.
4. Confirm metric/log retention and access control.
5. Approve the proposed SLO: 99.9% monthly external HTTPS availability, 99% of API requests below one second, fewer than 2% 5xx over ten minutes, backup age below 26 hours, and restore drill age within the approved cadence. Owner-approved targets in ADR-0007 supersede these proposals.
