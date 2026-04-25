<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 09 Testing and Acceptance Specification

## 1. Test types

Required:

- Unit tests
- Integration tests with recorded fixtures
- CLI tests
- Database migration tests
- Report snapshot tests
- Rule tests
- MCP tool tests

Optional:

- Live source tests gated behind environment variables
- E2E Docker tests

## 2. Fixtures

Store fixtures in:

```text
examples/fixtures/fmcsa_qcmobile/
examples/fixtures/socrata/
examples/fixtures/packets/
examples/fixtures/reports/
```

Fixture naming:

```text
{source}_{endpoint}_{case}.json
```

Examples:

```text
fmcsa_qcmobile_carrier_valid.json
fmcsa_qcmobile_docket_not_found.json
fmcsa_qcmobile_auth_invalid_key.json
socrata_company_census_sample.json
packet_basic_carrier_packet.txt
```

## 3. Unit test requirements

### Identifier normalization

Test:

```text
MC123456 -> type=mc value=123456
MC-123456 -> type=mc value=123456
mx 123456 -> type=mx value=123456
DOT 1234567 -> type=dot value=1234567
USDOT#1234567 -> type=dot value=1234567
```

### Phone normalization

Test:

```text
(555) 555-5555 -> +15555555555
555-555-5555 -> +15555555555
+1 555 555 5555 -> +15555555555
```

### Address normalization

Test common abbreviations:

```text
Street vs St
Road vs Rd
Suite vs Ste
case differences
extra whitespace
```

### Hashing

Test:

- Same normalized JSON with different key order produces same hash.
- Different material field produces different hash.
- Volatile `observed_at` does not change normalized hash.

## 4. CLI tests

Use a test harness that runs the compiled binary against a temporary OHG home.

Required tests:

```bash
ohg --version
ohg setup --quick --yes
ohg doctor --format json
ohg carrier lookup --dot 1234567 --offline
ohg carrier lookup --mc 123456 --format json
ohg carrier diff --mc 123456 --since 90d --format json
ohg watch add --mc 123456
ohg watch list --format json
ohg mcp serve --help
```

CLI JSON output must parse with standard JSON parser.

## 5. Source client tests

Use recorded fixtures by default. Live tests require env vars:

```text
OHG_TEST_LIVE_FMCSA=1
OHG_FMCSA_WEBKEY=...
OHG_TEST_LIVE_SOCRATA=1
OHG_SOCRATA_APP_TOKEN=...
```

Live tests must be skipped by default.

## 6. Scoring tests

Each rule must have:

- Positive case
- Negative case
- Boundary case
- Evidence shape test
- Recommendation mapping test

Example:

```text
NEW_AUTHORITY fires at 89 days
NEW_AUTHORITY does not fire at 90 days
VERY_NEW_AUTHORITY fires at 29 days
VERY_NEW_AUTHORITY suppresses NEW_AUTHORITY
```

## 7. Report snapshot tests

Snapshot test:

- Markdown carrier report
- JSON carrier report
- Table summary output
- Packet mismatch report
- Diff report

Snapshots must avoid timestamps or use fixed test clock.

## 8. MCP tests

Test tools with local fixture-backed app service.

Required:

- `carrier_lookup` returns schema-compatible JSON.
- `carrier_diff` returns field changes.
- `packet_check` handles missing file gracefully.
- MCP server does not expose secrets.

## 9. Acceptance test scenario

End-to-end scripted scenario:

```bash
export OHG_HOME=$(mktemp -d)
ohg setup --quick --yes
ohg doctor --format json
ohg carrier lookup --mc 123456 --format json --offline || true
ohg carrier lookup --mc 123456 --format json --fixture examples/fixtures/fmcsa_qcmobile/carrier_valid.json
ohg carrier lookup --mc 123456 --format markdown --fixture examples/fixtures/fmcsa_qcmobile/carrier_changed_phone.json
ohg carrier diff --mc 123456 --since 30d --format json
ohg watch add --mc 123456
ohg watch sync --fixture examples/fixtures/fmcsa_qcmobile/carrier_valid.json
ohg packet check examples/fixtures/packets/basic_packet.pdf --mc 123456 --format json
```

Note: `--fixture` is a test-only flag hidden from normal help output.

## 10. Platform matrix

Required for release:

```text
macOS arm64
macOS amd64
Linux amd64
Linux arm64
Windows amd64
```

## 11. Performance targets

Lazy lookup with live API:

```text
p95 under 5 seconds excluding source latency spikes
```

Cached lookup:

```text
p95 under 300 ms
```

Watchlist sync:

```text
Support 1,000 carriers in a single local run with rate limiting.
```

SQLite database:

```text
Support 100,000 observations without noticeable CLI slowdown for single-carrier lookup.
```

## 12. Security tests

- Secrets are not logged.
- Secrets are redacted in source_fetches URLs.
- Config fallback secrets file is `0600`.
- Packet content is not sent to external services.
- MCP server binds to stdio by default.
- HTTP MCP, if enabled, binds to `127.0.0.1` by default.
