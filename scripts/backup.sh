#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 /absolute/backup/directory" >&2
  exit 2
fi

backup_dir=$1
case "$backup_dir" in
  /*) ;;
  *) echo "backup directory must be absolute" >&2; exit 2 ;;
esac

if command -v hotelmate >/dev/null 2>&1; then
  exec hotelmate --backup-dir "$backup_dir" backup create --yes
fi
cd "$(dirname "$0")/../backend"
exec go run ./cmd/hotelmate --backup-dir "$backup_dir" backup create --yes
