package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Observation struct {
	ID             int64
	USDOTNumber    string
	ObservedAt     string
	NormalizedJSON string
	NormalizedHash string
	RawGroupID     string
}

type WatchEntry struct {
	ID              int64  `json:"id"`
	IdentifierType  string `json:"identifier_type"`
	IdentifierValue string `json:"identifier_value"`
	NormalizedValue string `json:"normalized_value"`
	USDOTNumber     string `json:"usdot_number,omitempty"`
	Label           string `json:"label,omitempty"`
	Active          bool   `json:"active"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	LastSyncedAt    string `json:"last_synced_at,omitempty"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);`); err != nil {
		return err
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if err := execScript(ctx, s.db, migration001); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)`, time.Now().UTC().Format(time.RFC3339))
	return err
}

func execScript(ctx context.Context, db *sql.DB, script string) error {
	for _, stmt := range strings.Split(script, ";\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration statement failed: %w: %s", err, stmt)
		}
	}
	return nil
}

func (s *Store) SetSetupState(ctx context.Context, key string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO setup_state(key, value_json, updated_at)
		VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json, updated_at=excluded.updated_at`,
		key, string(body), now)
	return err
}

func (s *Store) ResolveIdentifier(ctx context.Context, typ, normalizedValue string) (string, error) {
	if typ == "dot" {
		return normalizedValue, nil
	}
	var usdot string
	err := s.db.QueryRowContext(ctx, `
		SELECT usdot_number FROM carrier_identifiers
		WHERE identifier_type = ? AND normalized_value = ? AND usdot_number IS NOT NULL
		ORDER BY last_seen_at DESC LIMIT 1`, typ, normalizedValue).Scan(&usdot)
	return usdot, err
}

func (s *Store) SaveLookup(ctx context.Context, result domain.LookupResult, normalizedHash, rawGroupID string) (int64, int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	carrier := result.Carrier
	if carrier.SourceFirstSeenAt == "" {
		carrier.SourceFirstSeenAt = result.GeneratedAt
	}
	if carrier.LocalFirstSeenAt == "" {
		carrier.LocalFirstSeenAt = result.GeneratedAt
	}
	if carrier.LocalLastSeenAt == "" {
		carrier.LocalLastSeenAt = result.GeneratedAt
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO carriers(
			usdot_number, legal_name, dba_name, entity_type,
			physical_address_line1, physical_address_line2, physical_city, physical_state, physical_postal_code, physical_country,
			mailing_address_line1, mailing_address_line2, mailing_city, mailing_state, mailing_postal_code, mailing_country,
			phone, fax, email, power_units, drivers, mcs150_date,
			source_first_seen_at, local_first_seen_at, local_last_seen_at, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(usdot_number) DO UPDATE SET
			legal_name=excluded.legal_name,
			dba_name=excluded.dba_name,
			entity_type=excluded.entity_type,
			physical_address_line1=excluded.physical_address_line1,
			physical_address_line2=excluded.physical_address_line2,
			physical_city=excluded.physical_city,
			physical_state=excluded.physical_state,
			physical_postal_code=excluded.physical_postal_code,
			physical_country=excluded.physical_country,
			mailing_address_line1=excluded.mailing_address_line1,
			mailing_address_line2=excluded.mailing_address_line2,
			mailing_city=excluded.mailing_city,
			mailing_state=excluded.mailing_state,
			mailing_postal_code=excluded.mailing_postal_code,
			mailing_country=excluded.mailing_country,
			phone=excluded.phone,
			fax=excluded.fax,
			email=excluded.email,
			power_units=excluded.power_units,
			drivers=excluded.drivers,
			mcs150_date=excluded.mcs150_date,
			local_last_seen_at=excluded.local_last_seen_at,
			updated_at=excluded.updated_at`,
		carrier.USDOTNumber, carrier.LegalName, carrier.DBAName, carrier.EntityType,
		carrier.PhysicalAddress.Line1, carrier.PhysicalAddress.Line2, carrier.PhysicalAddress.City, carrier.PhysicalAddress.State, carrier.PhysicalAddress.PostalCode, carrier.PhysicalAddress.Country,
		carrier.MailingAddress.Line1, carrier.MailingAddress.Line2, carrier.MailingAddress.City, carrier.MailingAddress.State, carrier.MailingAddress.PostalCode, carrier.MailingAddress.Country,
		carrier.Contact.Phone, carrier.Contact.Fax, carrier.Contact.Email, carrier.Operations.PowerUnits, carrier.Operations.Drivers, carrier.Operations.MCS150Date,
		carrier.SourceFirstSeenAt, carrier.LocalFirstSeenAt, carrier.LocalLastSeenAt, now, now)
	if err != nil {
		return 0, 0, err
	}
	for _, id := range carrier.Identifiers {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO carrier_identifiers(usdot_number, identifier_type, identifier_value, normalized_value, status, source, first_seen_at, last_seen_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(identifier_type, normalized_value) DO UPDATE SET
				usdot_number=excluded.usdot_number,
				identifier_value=excluded.identifier_value,
				status=excluded.status,
				source=excluded.source,
				last_seen_at=excluded.last_seen_at`,
			carrier.USDOTNumber, strings.ToLower(id.Type), id.Value, digitsOnly(id.Value), id.Status, "lookup", result.GeneratedAt, result.GeneratedAt)
		if err != nil {
			return 0, 0, err
		}
	}
	if carrier.USDOTNumber != "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO carrier_identifiers(usdot_number, identifier_type, identifier_value, normalized_value, status, source, first_seen_at, last_seen_at)
			VALUES(?, 'dot', ?, ?, 'active', 'lookup', ?, ?)
			ON CONFLICT(identifier_type, normalized_value) DO UPDATE SET usdot_number=excluded.usdot_number, last_seen_at=excluded.last_seen_at`,
			carrier.USDOTNumber, carrier.USDOTNumber, carrier.USDOTNumber, result.GeneratedAt, result.GeneratedAt)
		if err != nil {
			return 0, 0, err
		}
	}
	for _, fetch := range result.Sources {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO source_fetches(raw_group_id, source_name, endpoint, request_method, request_url_redacted, status_code, fetched_at, duration_ms, response_hash, raw_path, error_code, error_message, created_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rawGroupID, fetch.SourceName, fetch.Endpoint, fetch.RequestMethod, fetch.RequestURLRedacted, fetch.StatusCode, fetch.FetchedAt, fetch.DurationMS, fetch.ResponseHash, fetch.RawPath, fetch.ErrorCode, fetch.ErrorMessage, now)
		if err != nil {
			return 0, 0, err
		}
	}
	body, err := json.Marshal(result.Carrier)
	if err != nil {
		return 0, 0, err
	}
	sourceSummary, err := json.Marshal(result.Sources)
	if err != nil {
		return 0, 0, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO carrier_observations(usdot_number, observed_at, observation_type, source_summary_json, normalized_json, normalized_hash, raw_group_id, created_at)
		VALUES(?, ?, 'lookup', ?, ?, ?, ?, ?)`,
		carrier.USDOTNumber, result.GeneratedAt, string(sourceSummary), string(body), normalizedHash, rawGroupID, now)
	if err != nil {
		return 0, 0, err
	}
	observationID, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}
	assessmentBody, err := json.Marshal(result.RiskAssessment)
	if err != nil {
		return 0, 0, err
	}
	res, err = tx.ExecContext(ctx, `
		INSERT INTO risk_assessments(usdot_number, observation_id, assessed_at, score, recommendation, assessment_json, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		carrier.USDOTNumber, observationID, result.GeneratedAt, result.RiskAssessment.Score, result.RiskAssessment.Recommendation, string(assessmentBody), now)
	if err != nil {
		return 0, 0, err
	}
	assessmentID, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}
	for _, flag := range result.RiskAssessment.Flags {
		evidence, err := json.Marshal(flag.Evidence)
		if err != nil {
			return 0, 0, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO risk_flags(assessment_id, code, severity, category, explanation, evidence_json, created_at)
			VALUES(?, ?, ?, ?, ?, ?, ?)`,
			assessmentID, flag.Code, flag.Severity, flag.Category, flag.Explanation, string(evidence), now)
		if err != nil {
			return 0, 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return observationID, assessmentID, nil
}

func (s *Store) LatestCarrierByIdentifier(ctx context.Context, typ, value string) (domain.CarrierProfile, Observation, error) {
	usdot, err := s.ResolveIdentifier(ctx, typ, value)
	if err != nil {
		if typ == "dot" {
			usdot = value
		} else {
			return domain.CarrierProfile{}, Observation{}, err
		}
	}
	return s.LatestCarrierByUSDOT(ctx, usdot)
}

func (s *Store) LatestCarrierByUSDOT(ctx context.Context, usdot string) (domain.CarrierProfile, Observation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, usdot_number, observed_at, normalized_json, normalized_hash, COALESCE(raw_group_id, '')
		FROM carrier_observations
		WHERE usdot_number = ?
		ORDER BY observed_at DESC, id DESC
		LIMIT 1`, usdot)
	var obs Observation
	if err := row.Scan(&obs.ID, &obs.USDOTNumber, &obs.ObservedAt, &obs.NormalizedJSON, &obs.NormalizedHash, &obs.RawGroupID); err != nil {
		return domain.CarrierProfile{}, Observation{}, err
	}
	var carrier domain.CarrierProfile
	if err := json.Unmarshal([]byte(obs.NormalizedJSON), &carrier); err != nil {
		return domain.CarrierProfile{}, Observation{}, err
	}
	return carrier, obs, nil
}

func (s *Store) ObservationsSince(ctx context.Context, typ, value string, since time.Time) ([]Observation, error) {
	usdot, err := s.ResolveIdentifier(ctx, typ, value)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, usdot_number, observed_at, normalized_json, normalized_hash, COALESCE(raw_group_id, '')
		FROM carrier_observations
		WHERE usdot_number = ? AND observed_at >= ?
		ORDER BY observed_at ASC, id ASC`, usdot, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var obs Observation
		if err := rows.Scan(&obs.ID, &obs.USDOTNumber, &obs.ObservedAt, &obs.NormalizedJSON, &obs.NormalizedHash, &obs.RawGroupID); err != nil {
			return nil, err
		}
		out = append(out, obs)
	}
	return out, rows.Err()
}

func (s *Store) ObservationCount(ctx context.Context, usdot string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM carrier_observations WHERE usdot_number = ?`, usdot).Scan(&count)
	return count, err
}

func (s *Store) AddWatch(ctx context.Context, typ, value, label, usdot string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO watchlist(identifier_type, identifier_value, normalized_value, usdot_number, label, active, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(identifier_type, normalized_value) DO UPDATE SET
			identifier_value=excluded.identifier_value,
			usdot_number=COALESCE(excluded.usdot_number, watchlist.usdot_number),
			label=excluded.label,
			active=1,
			updated_at=excluded.updated_at`,
		typ, value, value, nullEmpty(usdot), label, now, now)
	return err
}

func (s *Store) ListWatch(ctx context.Context) ([]WatchEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, identifier_type, identifier_value, normalized_value, COALESCE(usdot_number, ''), COALESCE(label, ''), active, created_at, updated_at, COALESCE(last_synced_at, '')
		FROM watchlist
		WHERE active = 1
		ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchEntry
	for rows.Next() {
		var item WatchEntry
		var active int
		if err := rows.Scan(&item.ID, &item.IdentifierType, &item.IdentifierValue, &item.NormalizedValue, &item.USDOTNumber, &item.Label, &active, &item.CreatedAt, &item.UpdatedAt, &item.LastSyncedAt); err != nil {
			return nil, err
		}
		item.Active = active == 1
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) MarkWatchSynced(ctx context.Context, id int64, usdot string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `UPDATE watchlist SET usdot_number = COALESCE(?, usdot_number), last_synced_at = ?, updated_at = ? WHERE id = ?`, nullEmpty(usdot), now, now, id)
	return err
}

func (s *Store) Healthy(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return err
	}
	var version int
	return s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func nullEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

const migration001 = `
CREATE TABLE IF NOT EXISTS carriers (
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
CREATE TABLE IF NOT EXISTS carrier_identifiers (
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
CREATE INDEX IF NOT EXISTS idx_carrier_identifiers_usdot ON carrier_identifiers(usdot_number);
CREATE TABLE IF NOT EXISTS carrier_observations (
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
CREATE INDEX IF NOT EXISTS idx_observations_usdot_time ON carrier_observations(usdot_number, observed_at);
CREATE INDEX IF NOT EXISTS idx_observations_hash ON carrier_observations(normalized_hash);
CREATE TABLE IF NOT EXISTS source_fetches (
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
CREATE INDEX IF NOT EXISTS idx_source_fetches_group ON source_fetches(raw_group_id);
CREATE INDEX IF NOT EXISTS idx_source_fetches_source_time ON source_fetches(source_name, fetched_at);
CREATE TABLE IF NOT EXISTS risk_assessments (
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
CREATE TABLE IF NOT EXISTS risk_flags (
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
CREATE INDEX IF NOT EXISTS idx_risk_flags_code ON risk_flags(code);
CREATE INDEX IF NOT EXISTS idx_risk_flags_severity ON risk_flags(severity);
CREATE TABLE IF NOT EXISTS watchlist (
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
CREATE TABLE IF NOT EXISTS packet_checks (
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
CREATE TABLE IF NOT EXISTS setup_state (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS source_metadata (
  source_name TEXT PRIMARY KEY,
  last_successful_sync_at TEXT,
  last_attempted_sync_at TEXT,
  last_error_code TEXT,
  last_error_message TEXT,
  metadata_json TEXT,
  updated_at TEXT NOT NULL
);
`
