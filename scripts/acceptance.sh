#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "usage: ACCEPTANCE_ONBOARDING_TOKEN=... $0 https://staging.hotel.example.com" >&2
  exit 2
fi

base_url=${1%/}
if command -v hotelmate >/dev/null 2>&1; then
  exec hotelmate --base-url "$base_url" acceptance
fi

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
exec go -C "$repo_dir/backend" run ./cmd/hotelmate --base-url "$base_url" acceptance
