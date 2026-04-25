# Server-side HTTP API integrations

These examples call a local `ohg serve` process from server-side code. They do
not use browser CORS, webhooks, or client-side secrets.

Start the API:

```bash
ohg serve --listen 127.0.0.1:8787
```

If the server was started with `OHG_API_TOKEN` or `--api-token`, export the same
token before running an example:

```bash
export OHG_API_TOKEN="replace-with-local-token"
```

Optional environment variables:

- `OHG_BASE_URL`, default `http://127.0.0.1:8787`
- `OHG_IDENTIFIER_TYPE`, default `mc`
- `OHG_IDENTIFIER_VALUE`, default `123456`

Run the examples:

```bash
bash examples/integrations/curl/lookup.sh
node examples/integrations/node/lookup.mjs
python3 examples/integrations/python/lookup.py
```
