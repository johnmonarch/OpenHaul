#!/usr/bin/env bash
set -euo pipefail

base_url="${OHG_BASE_URL:-http://127.0.0.1:8787}"
identifier_type="${OHG_IDENTIFIER_TYPE:-mc}"
identifier_value="${OHG_IDENTIFIER_VALUE:-123456}"

headers=(-H "Content-Type: application/json")
if [[ -n "${OHG_API_TOKEN:-}" ]]; then
  headers+=(-H "Authorization: Bearer ${OHG_API_TOKEN}")
fi

echo "GET ${base_url}/health"
curl -fsS "${base_url}/health"
echo

echo "POST ${base_url}/v1/carrier/lookup"
curl -fsS "${base_url}/v1/carrier/lookup" \
  "${headers[@]}" \
  -d @- <<JSON
{
  "identifier_type": "${identifier_type}",
  "identifier_value": "${identifier_value}",
  "max_age": "24h"
}
JSON
echo
