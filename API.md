# HTTP API

OpenHaul Guard can run a local HTTP API for website backends and internal tools:

```bash
ohg serve --listen 127.0.0.1:8787
```

The API is local-first and binds to `127.0.0.1` by default. It does not enable browser CORS. If you bind to a non-loopback address, configure a token:

```bash
OHG_API_TOKEN="$(openssl rand -hex 32)" ohg serve --listen 0.0.0.0:8787
```

Authenticated requests can use either header:

```text
Authorization: Bearer <token>
X-OpenHaul-Token: <token>
```

The OpenAPI 3 description is available at [`openapi/openapi.yaml`](openapi/openapi.yaml).
Small server-side examples for curl, Node.js, and Python are in
[`examples/integrations/`](examples/integrations/).
Service examples for systemd and launchd are in
[`docs/http-api-service.md`](docs/http-api-service.md).

## Endpoints

```text
GET  /health
POST /v1/carrier/lookup
POST /v1/carrier/diff
POST /v1/packet/extract
POST /v1/packet/check
GET  /v1/watch/export
```

Responses use the same JSON models as the CLI and MCP server. Error responses use:

```json
{
  "error": {
    "code": "OHG_INVALID_ARGS",
    "message": "identifier_type and identifier_value are required",
    "retryable": false
  }
}
```

## Examples

Health check:

```bash
curl -s http://127.0.0.1:8787/health
```

Carrier lookup:

```bash
curl -s http://127.0.0.1:8787/v1/carrier/lookup \
  -H 'Content-Type: application/json' \
  -d '{"identifier_type":"mc","identifier_value":"123456","max_age":"24h"}'
```

Carrier lookup with optional token auth:

```bash
headers=(-H 'Content-Type: application/json')
if [ -n "${OHG_API_TOKEN:-}" ]; then
  headers+=(-H "Authorization: Bearer ${OHG_API_TOKEN}")
fi

curl -s http://127.0.0.1:8787/v1/carrier/lookup \
  "${headers[@]}" \
  -d '{"identifier_type":"mc","identifier_value":"123456","max_age":"24h"}'
```

Carrier diff:

```bash
curl -s http://127.0.0.1:8787/v1/carrier/diff \
  -H 'Content-Type: application/json' \
  -d '{"identifier_type":"mc","identifier_value":"123456","since":"90d"}'
```

Packet extract:

```bash
curl -s http://127.0.0.1:8787/v1/packet/extract \
  -H 'Content-Type: application/json' \
  -d '{"path":"examples/fixtures/packets/basic_carrier_packet.txt"}'
```

Packet check:

```bash
curl -s http://127.0.0.1:8787/v1/packet/check \
  -H 'Content-Type: application/json' \
  -d '{"path":"examples/fixtures/packets/basic_carrier_packet.txt","identifier_type":"mc","identifier_value":"123456"}'
```

Watchlist export:

```bash
curl -s http://127.0.0.1:8787/v1/watch/export
```

## Integration Notes

Run the API on the same host as your website backend and call it from server-side code, not directly from a browser. Keep FMCSA and Socrata credentials on that machine. Webhooks are intentionally not included in the OSS API; use `watch sync`, `watch report --format json`, or `watch export --format json` from cron/systemd/custom scripts when event delivery is needed.
