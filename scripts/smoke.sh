#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "usage: $0 https://hotel.example.com" >&2
  exit 2
fi
base_url=${1%/}
case "$base_url" in
  http://*|https://*) ;;
  *) echo "base URL must start with http:// or https://" >&2; exit 2 ;;
esac

curl -fsS "$base_url/healthz" | grep -q '"status":"ok"'
curl -fsS "$base_url/readyz" | grep -q '"status":"ready"'
curl -fsS "$base_url/api/v1" | grep -q '"name":"HotelMate API"'

if [ -n "${SMOKE_HOTEL_SLUG:-}" ] || [ -n "${SMOKE_STAFF_EMAIL:-}" ] || [ -n "${SMOKE_STAFF_PASSWORD:-}" ]; then
  command -v jq >/dev/null 2>&1 || { echo "jq is required for authenticated smoke checks" >&2; exit 2; }
  [ -n "${SMOKE_HOTEL_SLUG:-}" ] && [ -n "${SMOKE_STAFF_EMAIL:-}" ] && [ -n "${SMOKE_STAFF_PASSWORD:-}" ] || { echo "set all SMOKE_HOTEL_SLUG, SMOKE_STAFF_EMAIL, and SMOKE_STAFF_PASSWORD" >&2; exit 2; }
  payload=$(jq -n --arg hotelSlug "$SMOKE_HOTEL_SLUG" --arg email "$SMOKE_STAFF_EMAIL" --arg password "$SMOKE_STAFF_PASSWORD" '{hotelSlug:$hotelSlug,email:$email,password:$password}')
  token=$(curl -fsS -H 'Content-Type: application/json' --data "$payload" "$base_url/api/v1/auth/staff/login" | jq -er '.token')
  curl -fsS -H "Authorization: Bearer $token" "$base_url/api/v1/staff/me" | jq -e '.staff.id and .hotel.id' >/dev/null
  curl -fsS -H "Authorization: Bearer $token" "$base_url/api/v1/staff/reports/operations" | jq -e '.report.summary' >/dev/null
fi

echo "HotelMate smoke checks passed for $base_url"
