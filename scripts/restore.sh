#!/bin/sh
set -eu

if [ "$#" -ne 2 ] || [ "$2" != "--confirm" ]; then
  echo "usage: $0 /absolute/path/hotelmate.dump --confirm" >&2
  echo "warning: this replaces matching objects in the configured production database" >&2
  exit 2
fi

backup_file=$1
case "$backup_file" in
  /*) ;;
  *) echo "backup path must be absolute" >&2; exit 2 ;;
esac
if [ ! -f "$backup_file" ]; then
  echo "backup file does not exist: $backup_file" >&2
  exit 2
fi

docker compose --env-file .env.production -f docker-compose.production.yml exec -T postgres \
  sh -c 'exec pg_restore --clean --if-exists --no-owner --no-privileges -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$backup_file"
echo "restore complete; run ./scripts/smoke.sh https://your-domain.example"
