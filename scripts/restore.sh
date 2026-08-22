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

if command -v hotelmate >/dev/null 2>&1; then
  exec hotelmate --manifest "$backup_file" backup restore --yes
fi
cd "$(dirname "$0")/../backend"
exec go run ./cmd/hotelmate --manifest "$backup_file" backup restore --yes
