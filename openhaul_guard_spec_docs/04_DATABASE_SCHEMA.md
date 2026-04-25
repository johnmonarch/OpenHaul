<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 04 Database Schema

## 1. Database

SQLite is the default local database.

Required pragmas:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
```

All timestamps are UTC ISO 8601 strings unless stored as integer epoch milliseconds. Prefer ISO 8601 text for readability.

## 2. Migration table

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
```

## 3. Core tables

### 3.1 carriers

Canonical carrier entity keyed by USDOT when known.

```sql
CREATE TABLE carriers (
  usdot_number TEXT PRIMARY KEY,
  legal_name TEXT,
  dba_name TEXT,
  entity_type TEXT,
  physical_address_line1 TEXT,
  physical_address_line2 TEXT,
  physical_city TEXT,
  physical_state TEXT,
  physical_postal_code TEXT,
  physical_country TEXT,
  mailing_address_line1 TEXT,
  mailing_address_line2 TEXT,
  mailing_city TEXT,
  mailing_state TEXT,
  mailing_postal_code TEXT,
  mailing_country TEXT,
  phone TEXT,
  fax TEXT,
  email TEXT,
  power_units INTEGER,
  drivers INTEGER,
  mcs150_date TEXT,
  source_first_seen_at TEXT NOT NULL,
  local_first_seen_at TEXT NOT NULL,
  local_last_seen_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

### 3.2 carrier_identifiers

MC/MX/FF/DOT and other identifiers.

```sql
CREATE TABLE carrier_identifiers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT,
  identifier_type TEXT NOT NULL,
  identifier_value TEXT NOT NULL,
  normalized_value TEXT NOT NULL,
  status TEXT,
  source TEXT,
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number),
  UNIQUE(identifier_type, normalized_value)
);

CREATE INDEX idx_carrier_identifiers_usdot ON carrier_identifiers(usdot_number);
```

### 3.3 carrier_observations

Point-in-time normalized snapshots.

```sql
CREATE TABLE carrier_observations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  observation_type TEXT NOT NULL DEFAULT 'lookup',
  source_summary_json TEXT NOT NULL,
  normalized_json TEXT NOT NULL,
  normalized_hash TEXT NOT NULL,
  raw_group_id TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number)
);

CREATE INDEX idx_observations_usdot_time ON carrier_observations(usdot_number, observed_at);
CREATE INDEX idx_observations_hash ON carrier_observations(normalized_hash);
```

### 3.4 source_fetches

```sql
CREATE TABLE source_fetches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  raw_group_id TEXT NOT NULL,
  source_name TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  request_method TEXT NOT NULL,
  request_url_redacted TEXT NOT NULL,
  status_code INTEGER,
  fetched_at TEXT NOT NULL,
  duration_ms INTEGER,
  response_hash TEXT,
  raw_path TEXT,
  error_code TEXT,
  error_message TEXT,
  created_at TEXT NOT NULL
);

CREATE INDEX idx_source_fetches_group ON source_fetches(raw_group_id);
CREATE INDEX idx_source_fetches_source_time ON source_fetches(source_name, fetched_at);
```

### 3.5 authority_records

```sql
CREATE TABLE authority_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT,
  docket_type TEXT,
  docket_number TEXT,
  authority_type TEXT,
  authority_status TEXT,
  original_action TEXT,
  original_action_date TEXT,
  final_action TEXT,
  final_action_date TEXT,
  source TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  raw_ref TEXT,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number)
);

CREATE INDEX idx_authority_usdot ON authority_records(usdot_number);
CREATE INDEX idx_authority_docket ON authority_records(docket_type, docket_number);
```

### 3.6 insurance_records

```sql
CREATE TABLE insurance_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT,
  docket_type TEXT,
  docket_number TEXT,
  insurance_type TEXT,
  insurer_name TEXT,
  policy_number TEXT,
  effective_date TEXT,
  cancellation_date TEXT,
  cancel_effective_date TEXT,
  cancellation_method TEXT,
  limit_amount TEXT,
  source TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  raw_ref TEXT,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number)
);

CREATE INDEX idx_insurance_usdot ON insurance_records(usdot_number);
CREATE INDEX idx_insurance_docket ON insurance_records(docket_type, docket_number);
```

### 3.7 boc3_records

```sql
CREATE TABLE boc3_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT,
  docket_type TEXT,
  docket_number TEXT,
  agent_name TEXT,
  agent_address TEXT,
  source TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  raw_ref TEXT,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number)
);
```

### 3.8 sms_records

```sql
CREATE TABLE sms_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT NOT NULL,
  sms_month TEXT NOT NULL,
  basic_category TEXT NOT NULL,
  measure TEXT,
  percentile TEXT,
  alert_status TEXT,
  source TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  raw_ref TEXT,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number)
);

CREATE INDEX idx_sms_usdot_month ON sms_records(usdot_number, sms_month);
```

### 3.9 risk_assessments

```sql
CREATE TABLE risk_assessments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT NOT NULL,
  observation_id INTEGER,
  assessed_at TEXT NOT NULL,
  score INTEGER,
  recommendation TEXT NOT NULL,
  assessment_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number),
  FOREIGN KEY (observation_id) REFERENCES carrier_observations(id)
);
```

### 3.10 risk_flags

```sql
CREATE TABLE risk_flags (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  assessment_id INTEGER NOT NULL,
  code TEXT NOT NULL,
  severity TEXT NOT NULL,
  category TEXT NOT NULL,
  explanation TEXT NOT NULL,
  evidence_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (assessment_id) REFERENCES risk_assessments(id)
);

CREATE INDEX idx_risk_flags_code ON risk_flags(code);
CREATE INDEX idx_risk_flags_severity ON risk_flags(severity);
```

### 3.11 watchlist

```sql
CREATE TABLE watchlist (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  identifier_type TEXT NOT NULL,
  identifier_value TEXT NOT NULL,
  normalized_value TEXT NOT NULL,
  usdot_number TEXT,
  label TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_synced_at TEXT,
  UNIQUE(identifier_type, normalized_value)
);
```

### 3.12 packet_checks

```sql
CREATE TABLE packet_checks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usdot_number TEXT,
  packet_path TEXT NOT NULL,
  packet_hash TEXT NOT NULL,
  extracted_json TEXT NOT NULL,
  comparison_json TEXT NOT NULL,
  checked_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (usdot_number) REFERENCES carriers(usdot_number)
);
```

### 3.13 setup_state

```sql
CREATE TABLE setup_state (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

### 3.14 source_metadata

```sql
CREATE TABLE source_metadata (
  source_name TEXT PRIMARY KEY,
  last_successful_sync_at TEXT,
  last_attempted_sync_at TEXT,
  last_error_code TEXT,
  last_error_message TEXT,
  metadata_json TEXT,
  updated_at TEXT NOT NULL
);
```

## 4. Canonical JSON models

### 4.1 CarrierProfile

```json
{
  "usdot_number": "1234567",
  "legal_name": "EXAMPLE TRUCKING LLC",
  "dba_name": null,
  "identifiers": [
    {"type": "MC", "value": "123456", "status": "active"}
  ],
  "entity_type": "carrier",
  "addresses": {
    "physical": {},
    "mailing": {}
  },
  "contact": {
    "phone": "+15555555555",
    "email": null
  },
  "operations": {
    "power_units": 12,
    "drivers": 14,
    "operation_classification": []
  },
  "authority": [],
  "insurance": [],
  "safety": {}
}
```

### 4.2 RiskAssessment

```json
{
  "score": 42,
  "recommendation": "manual_review_recommended",
  "flags": [
    {
      "code": "NEW_AUTHORITY",
      "severity": "medium",
      "category": "authority",
      "explanation": "Operating authority appears to be less than 90 days old.",
      "evidence": [
        {
          "field": "authority.original_action_date",
          "value": "2026-04-01",
          "source": "fmcsa_qcmobile_authority"
        }
      ],
      "confidence": "medium"
    }
  ]
}
```

## 5. Hashing

Use SHA-256.

Raw hash:

- Hash raw response body bytes.

Normalized hash:

- Canonicalize JSON by sorting keys.
- Remove volatile fields such as `observed_at`.
- Hash canonical JSON bytes.

## 6. PII and privacy

Some public records contain names, addresses, phone numbers, and emails. These are public-record fields, but local tool behavior must be conservative.

Requirements:

- Store local data only by default.
- Do not send local packet contents to third-party services.
- Do not enable telemetry by default.
- Redact secrets from logs.
- Allow user to delete local data.

Command:

```bash
ohg data purge --mc 123456
ohg data purge --all
```

`data purge` can be post-MVP but schema should allow deletion.
