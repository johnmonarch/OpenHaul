package domain

import "time"

const SchemaVersion = "1.0"

type Address struct {
	Line1      string `json:"line1,omitempty"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"`
}

type Identifier struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Status string `json:"status,omitempty"`
}

type Contact struct {
	Phone string `json:"phone,omitempty"`
	Fax   string `json:"fax,omitempty"`
	Email string `json:"email,omitempty"`
}

type AuthorityRecord struct {
	DocketType         string `json:"docket_type,omitempty"`
	DocketNumber       string `json:"docket_number,omitempty"`
	AuthorityType      string `json:"authority_type,omitempty"`
	AuthorityStatus    string `json:"authority_status,omitempty"`
	OriginalAction     string `json:"original_action,omitempty"`
	OriginalActionDate string `json:"original_action_date,omitempty"`
	FinalAction        string `json:"final_action,omitempty"`
	FinalActionDate    string `json:"final_action_date,omitempty"`
	Source             string `json:"source,omitempty"`
	ObservedAt         string `json:"observed_at,omitempty"`
}

type InsuranceRecord struct {
	InsuranceType       string `json:"insurance_type,omitempty"`
	InsurerName         string `json:"insurer_name,omitempty"`
	PolicyNumber        string `json:"policy_number,omitempty"`
	EffectiveDate       string `json:"effective_date,omitempty"`
	CancellationDate    string `json:"cancellation_date,omitempty"`
	CancelEffectiveDate string `json:"cancel_effective_date,omitempty"`
	Source              string `json:"source,omitempty"`
	ObservedAt          string `json:"observed_at,omitempty"`
}

type Operations struct {
	PowerUnits              int      `json:"power_units,omitempty"`
	Drivers                 int      `json:"drivers,omitempty"`
	OperationClassification []string `json:"operation_classification,omitempty"`
	CargoCarried            []string `json:"cargo_carried,omitempty"`
	MCS150Date              string   `json:"mcs150_date,omitempty"`
}

type Safety struct {
	OutOfServiceStatus string `json:"out_of_service_status,omitempty"`
	SMSMonth           string `json:"sms_month,omitempty"`
}

type CarrierProfile struct {
	USDOTNumber       string            `json:"usdot_number"`
	LegalName         string            `json:"legal_name,omitempty"`
	DBAName           string            `json:"dba_name,omitempty"`
	Identifiers       []Identifier      `json:"identifiers,omitempty"`
	EntityType        string            `json:"entity_type,omitempty"`
	PhysicalAddress   Address           `json:"physical_address,omitempty"`
	MailingAddress    Address           `json:"mailing_address,omitempty"`
	Contact           Contact           `json:"contact,omitempty"`
	Operations        Operations        `json:"operations,omitempty"`
	Authority         []AuthorityRecord `json:"authority,omitempty"`
	Insurance         []InsuranceRecord `json:"insurance,omitempty"`
	Safety            Safety            `json:"safety,omitempty"`
	SourceFirstSeenAt string            `json:"source_first_seen_at,omitempty"`
	LocalFirstSeenAt  string            `json:"local_first_seen_at,omitempty"`
	LocalLastSeenAt   string            `json:"local_last_seen_at,omitempty"`
}

type SourceFetchResult struct {
	SourceName         string `json:"source_name"`
	Endpoint           string `json:"endpoint"`
	RequestMethod      string `json:"request_method"`
	RequestURLRedacted string `json:"request_url_redacted"`
	StatusCode         int    `json:"status_code,omitempty"`
	FetchedAt          string `json:"fetched_at"`
	DurationMS         int64  `json:"duration_ms,omitempty"`
	ResponseHash       string `json:"response_hash,omitempty"`
	RawPath            string `json:"raw_path,omitempty"`
	ErrorCode          string `json:"error_code,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
}

type Evidence struct {
	Field           string `json:"field"`
	Value           any    `json:"value,omitempty"`
	SourceValue     any    `json:"source_value,omitempty"`
	ComparisonValue any    `json:"comparison_value,omitempty"`
	Source          string `json:"source"`
	ObservedAt      string `json:"observed_at,omitempty"`
}

type RiskFlag struct {
	Code         string     `json:"code"`
	Severity     string     `json:"severity"`
	Category     string     `json:"category"`
	Explanation  string     `json:"explanation"`
	WhyItMatters string     `json:"why_it_matters"`
	NextStep     string     `json:"next_step"`
	Evidence     []Evidence `json:"evidence"`
	Confidence   string     `json:"confidence"`
}

type RiskAssessment struct {
	ID             int64      `json:"id,omitempty"`
	Score          int        `json:"score"`
	Recommendation string     `json:"recommendation"`
	Flags          []RiskFlag `json:"flags"`
}

type FreshnessItem struct {
	Source    string `json:"source"`
	FetchedAt string `json:"fetched_at,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

type FreshnessSummary struct {
	Mode    string          `json:"mode"`
	Sources []FreshnessItem `json:"sources"`
}

type UserWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

type LookupRequest struct {
	IdentifierType  string
	IdentifierValue string
	ForceRefresh    bool
	Offline         bool
	MaxAge          time.Duration
	FixturePath     string
}

type LookupInfo struct {
	InputType      string `json:"input_type"`
	InputValue     string `json:"input_value"`
	ResolvedUSDOT  string `json:"resolved_usdot,omitempty"`
	LookedUpAt     string `json:"looked_up_at"`
	Mode           string `json:"mode"`
	LocalFirstSeen string `json:"local_first_seen_at,omitempty"`
}

type LookupResult struct {
	SchemaVersion  string              `json:"schema_version"`
	ReportType     string              `json:"report_type"`
	GeneratedAt    string              `json:"generated_at"`
	Lookup         LookupInfo          `json:"lookup"`
	Carrier        CarrierProfile      `json:"carrier"`
	Freshness      FreshnessSummary    `json:"freshness"`
	RiskAssessment RiskAssessment      `json:"risk_assessment"`
	Sources        []SourceFetchResult `json:"sources"`
	Warnings       []UserWarning       `json:"warnings,omitempty"`
	Disclaimer     string              `json:"disclaimer"`
}

type FieldDiff struct {
	FieldPath          string `json:"field_path"`
	PreviousValue      string `json:"previous_value"`
	CurrentValue       string `json:"current_value"`
	PreviousObservedAt string `json:"previous_observed_at"`
	CurrentObservedAt  string `json:"current_observed_at"`
	Material           bool   `json:"material"`
}

type DiffResult struct {
	SchemaVersion    string      `json:"schema_version"`
	ReportType       string      `json:"report_type"`
	GeneratedAt      string      `json:"generated_at"`
	IdentifierType   string      `json:"identifier_type"`
	IdentifierValue  string      `json:"identifier_value"`
	ResolvedUSDOT    string      `json:"resolved_usdot,omitempty"`
	ObservationCount int         `json:"observation_count"`
	Changes          []FieldDiff `json:"changes"`
}

const Disclaimer = "OpenHaul Guard does not label carriers as fraudulent and does not make tendering decisions. Use this report as part of a human compliance process."
