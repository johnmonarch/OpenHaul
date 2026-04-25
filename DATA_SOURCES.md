# Data Sources

OpenHaul Guard keeps source access local to the user. Live API credentials belong to the user and are not bundled with the project.

## Currently Used

### FMCSA QCMobile API

Live carrier lookup uses FMCSA QCMobile when a WebKey is configured.

Base URL:

```text
https://mobile.fmcsa.dot.gov/qc/services
```

Current lookup endpoints:

```text
GET /carriers/docket-number/:docketNumber/
GET /carriers/:dotNumber
GET /carriers/:dotNumber/basics
GET /carriers/:dotNumber/authority
GET /carriers/:dotNumber/docket-numbers
GET /carriers/:dotNumber/oos
```

The WebKey is passed as the `webKey` query parameter and redacted from stored source metadata.

Configure it with:

```bash
ohg setup fmcsa
```

### Local SQLite Cache

Lookups store normalized carrier observations, raw source metadata, risk assessments, and watchlist entries in SQLite. Repeated lookup can use cache when the latest observation is younger than `--max-age`.

Offline mode only reads local data:

```bash
ohg carrier lookup --mc 123456 --offline
```

If no local observation exists, offline mode returns `OHG_OFFLINE_CACHE_MISS`.

### Packet Files

Packet checks read `.txt`, `.text`, extensionless text, or text-based `.pdf` files. PDF extraction requires `pdftotext`. The checker extracts fields from the packet, runs a carrier lookup, and compares packet values against public-record lookup values.

Compared fields include:

```text
legal_name
dba_name
usdot_number
physical_address
phone
email
identifier.mc
identifier.mx
identifier.ff
```

### Test Fixtures

The CLI has a hidden `--fixture` flag for development and tests:

```bash
ohg carrier lookup --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json
```

Fixture lookups are persisted like other observations and are useful for local verification without live credentials.

## Partially Implemented

The repository includes a Socrata client and a registry entry for the DOT DataHub Company Census File (`az4n-8mr2`). The current carrier lookup path still uses FMCSA QCMobile, local cache, or fixtures.

The config model also includes bootstrap mirror settings, but this build does not download or query the mirror.

## Freshness and Interpretation

Reports include source freshness information. Public data may lag real-world status, and OpenHaul Guard risk flags are interpretations for manual review. The tool does not declare carriers fraudulent and does not make tendering decisions.
