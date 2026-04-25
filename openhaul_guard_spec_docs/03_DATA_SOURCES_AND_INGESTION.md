<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 03 Data Sources and Ingestion Specification

## 1. Source strategy

OpenHaul Guard must support three source tiers:

1. Live official lookup APIs
2. Public bulk datasets
3. Optional OpenHaul bootstrap mirror

The first useful lookup should use live official lookup APIs when possible. Bulk sync is optional.

## 2. Official sources

### 2.1 FMCSA QCMobile API

Purpose:

- Live carrier lookup
- DOT lookup
- Docket lookup for MC/MX/FF-like identifiers
- Basics
- Cargo carried
- Operation classification
- Out-of-service data
- Docket numbers
- Authority data

Base URL:

```text
https://mobile.fmcsa.dot.gov/qc/services/
```

Registration:

Required for live calls. FMCSA requires a developer account and WebKey. Official API access instructions say the user must log in with Login.gov, select `My WebKeys`, click `Get a new WebKey`, and append the key as `webKey=your_web_key` in calls.

Official docs:

```text
https://mobile.fmcsa.dot.gov/QCDevsite/docs/apiAccess
https://mobile.fmcsa.dot.gov/QCDevsite/docs/qcApi
```

Important endpoints from official docs:

```text
GET /carriers/name/:name?webKey=...
GET /carriers/:dotNumber?webKey=...
GET /carriers/docket-number/:docketNumber/?webKey=...
GET /carriers/:dotNumber/basics?webKey=...
GET /carriers/:dotNumber/cargo-carried?webKey=...
GET /carriers/:dotNumber/operation-classification?webKey=...
GET /carriers/:dotNumber/oos?webKey=...
GET /carriers/:dotNumber/docket-numbers?webKey=...
GET /carriers/:dotNumber/authority?webKey=...
```

Implementation requirement:

- Store the WebKey securely.
- Never print the WebKey.
- Validate the WebKey using a low-cost known query.
- Treat 401 as invalid key.
- Treat 404 as carrier/source not found, not a system failure.
- Apply rate limiting and retries even if the docs do not publish specific limits.

### 2.2 DOT DataHub and Socrata APIs

Purpose:

- Bulk data sync
- Company Census File
- Operating Authority history files
- Insurance history
- BOC-3 history
- Revocation history
- SMS monthly data
- Crash and inspection data where public and appropriate

Registration:

Socrata app token is optional for some simple unauthenticated SODA 2.x reads, but should be strongly recommended. Socrata says app tokens provide higher throttling limits and should be sent with the `X-App-Token` header for SODA 3.0 and 2.x. SODA 3 query requests require authentication or a valid app token.

Official docs:

```text
https://dev.socrata.com/docs/app-tokens.html
https://dev.socrata.com/docs/endpoints.html
https://data.transportation.gov/
```

Known dataset page:

```text
Company Census File: https://data.transportation.gov/Trucking-and-Motorcoaches/Company-Census-File/az4n-8mr2
```

FMCSA Open Data Program reference:

```text
https://www.fmcsa.dot.gov/registration/fmcsa-data-dissemination-program
```

Implementation requirement:

- Support SODA 2.x resource endpoints where available.
- Support SODA 3 query endpoints where required.
- Send `X-App-Token` when token is configured.
- Clearly tell user that token is optional for quick lookup but recommended for sync reliability.
- Dataset identifiers must not be hardcoded without a discovery/update mechanism. Maintain a registry file and allow updates.

### 2.3 FMCSA Open Data Program datasets

FMCSA says its open data program provides no-cost regulated entity census and safety performance data through DOT DataHub. FMCSA organizes datasets into:

- Entities with a USDOT Number
- Entities with Operating Authority
- FMCSA Safety Measurement System Data for active motor carriers
- New Entrant Safety Assurance Program OOS Orders

Important refresh notes:

- Company Census File is updated daily from a 24-hour-old database.
- Entities with Operating Authority files are updated from a 24-hour-old database.
- SMS data refreshes monthly.

Implementation requirement:

- Always expose source freshness in reports.
- Never imply daily data is real-time.
- Never imply SMS monthly data is current to the day.

## 3. Optional OpenHaul bootstrap mirror

Purpose:

Reduce setup friction.

The mirror is a public, OSS-maintained, redistributable index containing only fields that can be redistributed from public data sources.

MVP mirror contents:

```text
mc_number,docket_type,usdot_number,legal_name,dba_name,last_source_update
```

Use cases:

- Resolve MC to USDOT before live key is configured.
- Enable quick setup mode.
- Provide helpful messages before users configure official keys.

Constraints:

- Mirror must include source timestamp and build timestamp.
- Mirror must not include proprietary data.
- Mirror must not be represented as real-time.
- Mirror must be replaceable by user-maintained mirrors.

Config:

```toml
[sources.mirror]
enabled = true
url = "https://downloads.openhaulguard.org/bootstrap/mc_dot_index.parquet"
checksum_url = "https://downloads.openhaulguard.org/bootstrap/mc_dot_index.sha256"
```

Formats:

- Prefer compressed Parquet for full index.
- Provide CSV fallback.
- Use SQLite import for local index.

## 4. Ingestion modes

### 4.1 Lazy lookup

Default mode.

Command:

```bash
ohg carrier lookup --mc 123456
```

Flow:

```text
Normalize input
Check local identifier index
If unresolved, try bootstrap mirror
If still unresolved and FMCSA key exists, call QCMobile docket endpoint
Resolve USDOT
Check observation cache freshness
If stale or missing, call QCMobile carrier endpoints
Normalize data
Store raw payloads
Store normalized observation
Run risk rules
Return report
```

### 4.2 Offline lookup

Command:

```bash
ohg carrier lookup --mc 123456 --offline
```

Flow:

```text
Normalize input
Search local identifiers and mirror index
Search existing carrier profile
If found, return local report with freshness warning
If not found, return OHG_OFFLINE_CACHE_MISS
```

### 4.3 Watchlist sync

Command:

```bash
ohg watch sync
```

Flow:

```text
Load watchlist
For each item:
  run lazy lookup with force refresh if requested
  persist new observation
  compute diff against previous observation
  run risk rules
  store sync result
Print summary
```

### 4.4 Full sync

Post-MVP or advanced MVP.

Command:

```bash
ohg sync full
```

Flow:

```text
Validate Socrata token if configured
Load dataset registry
Download or query Company Census File
Download selected operating authority history files
Import into staging tables
Validate row counts and hashes where possible
Update identifier index
Update source metadata
Vacuum/analyze local database
```

## 5. Cache freshness

Defaults:

```text
QCMobile live carrier profile: 24 hours
QCMobile authority endpoint: 24 hours
Bootstrap mirror index: 7 days warning, 30 days stale
Company Census bulk sync: 24 hours expected source cadence
Operating Authority history bulk sync: 24 hours expected source cadence
SMS data: monthly source cadence
```

Commands:

```bash
ohg carrier lookup --mc 123456 --max-age 6h
ohg carrier lookup --mc 123456 --force-refresh
ohg carrier lookup --mc 123456 --offline
```

## 6. Raw payload storage

For every network response, store:

- Source name
- Endpoint
- Request metadata with secrets redacted
- HTTP status
- Response headers allowlist
- Fetched timestamp
- SHA-256 hash
- Raw body path

Raw path format:

```text
~/.openhaulguard/raw/{source}/{entity}/{YYYY}/{MM}/{DD}/{timestamp}_{hash}.json
```

Example:

```text
~/.openhaulguard/raw/fmcsa_qcmobile/carrier_1234567/2026/04/25/20260425T183000Z_abcd1234.json
```

## 7. Dataset registry

Create:

```text
internal/sources/registry/datasets.json
```

Example:

```json
{
  "company_census": {
    "source": "dot_datahub",
    "title": "Company Census File",
    "dataset_id": "az4n-8mr2",
    "url": "https://data.transportation.gov/Trucking-and-Motorcoaches/Company-Census-File/az4n-8mr2",
    "required": false,
    "refresh_cadence": "daily_24h_lag"
  }
}
```

The registry must be updateable without recompiling by placing a user override at:

```text
~/.openhaulguard/datasets.override.json
```

## 8. Source reliability rules

- Use official source APIs first when configured.
- If official API fails and local cache exists, return cached data with warning.
- If official API fails and mirror data exists, return mirror data with warning.
- If no source can answer, return clear user action.

Example error:

```text
I could not complete a live FMCSA lookup because your FMCSA WebKey is missing.
Run: ohg setup fmcsa
Or use offline mode if you already synced this carrier: ohg carrier lookup --mc 123456 --offline
```

## 9. Respectful usage

Implement:

- Per-source rate limiter
- Exponential backoff with jitter
- Retry only idempotent GET requests
- Stop retries on 401, 403, 404
- Obey 429 retry headers when present
- Use descriptive User-Agent

User-Agent format:

```text
OpenHaulGuard/{version} (+https://github.com/openhaulguard/openhaulguard)
```
