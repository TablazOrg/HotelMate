#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "usage: $0 https://hotel.example.com" >&2
  exit 2
fi
base_url=${1%/}
if command -v hotelmate >/dev/null 2>&1; then
  exec hotelmate --base-url "$base_url" smoke
fi
cd "$(dirname "$0")/../backend"
exec go run ./cmd/hotelmate --base-url "$base_url" smoke
