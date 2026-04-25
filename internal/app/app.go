package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/config"
	"github.com/openhaulguard/openhaulguard/internal/credentials"
	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/normalize"
	"github.com/openhaulguard/openhaulguard/internal/scoring"
	"github.com/openhaulguard/openhaulguard/internal/sources/fmcsa"
	"github.com/openhaulguard/openhaulguard/internal/store"
	"github.com/openhaulguard/openhaulguard/internal/version"
	"golang.org/x/term"
)

type App struct {
	Config config.Config
	Creds  credentials.Store
	Store  *store.Store
}

type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

type DoctorResult struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	Status        string        `json:"status"`
	Checks        []DoctorCheck `json:"checks"`
	NextStep      string        `json:"next_step"`
}

type Options struct {
	Home       string
	ConfigPath string
	DBPath     string
}

func New(ctx context.Context, opts Options, migrate bool) (*App, error) {
	cfg, err := config.Load(config.Overrides{Home: opts.Home, ConfigPath: opts.ConfigPath, DBPath: opts.DBPath})
	if err != nil {
		return nil, err
	}
	app := &App{Config: cfg, Creds: credentials.Store{Home: cfg.Home}}
	if migrate {
		if err := cfg.EnsureDirs(); err != nil {
			return nil, err
		}
		st, err := store.Open(cfg.DBPath)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CodeDatabase, "could not open local SQLite database", "Run: ohg setup --quick", err)
		}
		if err := st.Migrate(ctx); err != nil {
			_ = st.Close()
			return nil, apperrors.Wrap(apperrors.CodeDatabase, "could not run database migrations", "Run: ohg doctor", err)
		}
		app.Store = st
	}
	return app, nil
}

func (a *App) Close() error {
	if a.Store != nil {
		return a.Store.Close()
	}
	return nil
}

func (a *App) SetupQuick(ctx context.Context) error {
	if err := a.Config.EnsureDirs(); err != nil {
		return err
	}
	if err := a.Config.Save(); err != nil {
		return err
	}
	if a.Store == nil {
		st, err := store.Open(a.Config.DBPath)
		if err != nil {
			return err
		}
		a.Store = st
	}
	if err := a.Store.Migrate(ctx); err != nil {
		return err
	}
	_ = a.Store.SetSetupState(ctx, "database_initialized", true)
	_ = a.Store.SetSetupState(ctx, "quick_setup_complete", true)
	return nil
}

func (a *App) SetupCredential(ctx context.Context, kind string, noBrowser bool, provided string) error {
	var keyUser, urlToOpen, label string
	switch kind {
	case "fmcsa":
		keyUser = credentials.UserFMCSAWebKey
		urlToOpen = "https://mobile.fmcsa.dot.gov/QCDevsite/docs/apiAccess"
		label = "FMCSA WebKey"
	case "socrata":
		keyUser = credentials.UserSocrataAppToken
		urlToOpen = "https://data.transportation.gov/profile/edit/developer_settings"
		label = "Socrata app token"
	default:
		return apperrors.New(apperrors.CodeInvalidArgs, "unknown credential setup target", "")
	}
	if provided == "" {
		fmt.Printf("%s setup\n\n", label)
		if !noBrowser {
			fmt.Printf("Press Enter to open %s setup in your browser, or type s to skip: ", label)
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(strings.TrimSpace(response)) != "s" {
				if err := openURL(urlToOpen); err != nil {
					fmt.Printf("I could not open a browser from this terminal.\nOpen this URL:\n%s\n\n", urlToOpen)
				}
			}
		} else {
			fmt.Printf("Open this URL in your browser:\n%s\n\n", urlToOpen)
		}
		fmt.Printf("Paste %s. Input will be hidden when the terminal supports it:\n> ", label)
		secret, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			var fallback string
			fmt.Scanln(&fallback)
			provided = strings.TrimSpace(fallback)
		} else {
			provided = strings.TrimSpace(string(secret))
		}
	}
	if provided == "" {
		return apperrors.New(apperrors.CodeInvalidArgs, "empty credential was not stored", "")
	}
	if kind == "fmcsa" {
		client := fmcsa.New(provided, version.Version)
		if err := client.ValidateWebKey(ctx); err != nil {
			return apperrors.Wrap(apperrors.CodeAuthFMCSAInvalid, "that FMCSA WebKey did not validate", "Make sure you copied the WebKey, not the client secret, then run: ohg setup fmcsa", err)
		}
	}
	if err := a.Creds.Set(keyUser, provided); err != nil {
		return err
	}
	if a.Store != nil {
		_ = a.Store.SetSetupState(ctx, kind+"_credential_validated", true)
	}
	return nil
}

func (a *App) Doctor(ctx context.Context) DoctorResult {
	result := DoctorResult{
		SchemaVersion: domain.SchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Status:        "usable",
		NextStep:      "ohg carrier lookup --mc 123456",
	}
	add := func(name, status, msg, fix string) {
		if status == "fail" {
			result.Status = "needs_attention"
		}
		result.Checks = append(result.Checks, DoctorCheck{Name: name, Status: status, Message: msg, Fix: fix})
	}
	if _, err := os.Stat(a.Config.Home); err == nil {
		add("OHG home directory", "ok", a.Config.Home, "")
	} else {
		add("OHG home directory", "fail", err.Error(), "ohg setup --quick")
	}
	if _, err := os.Stat(a.Config.Path); err == nil {
		add("Config file", "ok", a.Config.Path, "")
	} else {
		add("Config file", "fail", err.Error(), "ohg setup --quick")
	}
	if a.Store != nil && a.Store.Healthy(ctx) == nil {
		add("SQLite database", "ok", a.Config.DBPath, "")
	} else {
		add("SQLite database", "fail", "database is missing or migrations are not current", "ohg setup --quick")
	}
	if _, err := os.Stat(a.Config.RawDir); err == nil {
		add("Raw payload directory", "ok", a.Config.RawDir, "")
	} else {
		add("Raw payload directory", "fail", err.Error(), "ohg setup --quick")
	}
	if _, err := a.Creds.Get(credentials.UserFMCSAWebKey); err == nil {
		add("FMCSA WebKey", "ok", "credential present", "")
	} else {
		add("FMCSA WebKey", "warn", "missing, live FMCSA lookup will not work", "ohg setup fmcsa")
	}
	if _, err := a.Creds.Get(credentials.UserSocrataAppToken); err == nil {
		add("Socrata app token", "ok", "credential present", "")
	} else {
		add("Socrata app token", "warn", "missing, optional for quick lookup", "ohg setup socrata")
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		add("PDF extraction", "ok", "pdftotext available", "")
	} else {
		add("PDF extraction", "warn", "pdftotext not found; packet checks may be limited", "Install poppler/pdftotext")
	}
	add("MCP server", "ok", "stdio mode available", "")
	return result
}

func (a *App) Lookup(ctx context.Context, req domain.LookupRequest) (domain.LookupResult, error) {
	typ, value, err := normalize.Identifier(req.IdentifierType, req.IdentifierValue)
	if err != nil {
		return domain.LookupResult{}, apperrors.Wrap(apperrors.CodeInvalidArgs, "carrier lookup requires exactly one valid identifier", "", err)
	}
	req.IdentifierType = typ
	req.IdentifierValue = value
	if req.MaxAge == 0 {
		req.MaxAge = a.Config.MaxAgeDuration()
	}
	if req.FixturePath != "" {
		return a.lookupFixture(ctx, req)
	}
	if req.Offline {
		return a.lookupOffline(ctx, req)
	}
	if !req.ForceRefresh {
		if result, ok := a.lookupFreshCache(ctx, req); ok {
			return result, nil
		}
	}
	webKey, err := a.Creds.Get(credentials.UserFMCSAWebKey)
	if err != nil || webKey == "" {
		if result, ok := a.lookupFreshCache(ctx, req); ok {
			result.Warnings = append(result.Warnings, domain.UserWarning{
				Code:    "OHG_AUTH_FMCSA_MISSING",
				Message: "Live FMCSA lookup needs a free FMCSA WebKey, so this report uses local cache.",
				Action:  "Run: ohg setup fmcsa",
			})
			return result, nil
		}
		return domain.LookupResult{}, apperrors.New(apperrors.CodeAuthFMCSAMissing, "live FMCSA lookup requires a FMCSA WebKey", "Run: ohg setup fmcsa")
	}
	client := fmcsa.New(webKey, version.Version)
	var raws []fmcsa.RawResponse
	dot := value
	if typ != "dot" {
		docket, err := client.Docket(ctx, value)
		raws = append(raws, docket)
		if err != nil {
			return domain.LookupResult{}, mapSourceErr(err)
		}
		dot = extractDOTFromRaw(docket.Body)
		if dot == "" {
			return domain.LookupResult{}, apperrors.New(apperrors.CodeSourceNotFound, "FMCSA did not resolve that docket number to a USDOT number", "")
		}
	}
	for _, fetch := range []func(context.Context, string) (fmcsa.RawResponse, error){
		client.Carrier,
		client.Basics,
		client.Authority,
		client.DocketNumbers,
		client.OOS,
	} {
		raw, err := fetch(ctx, dot)
		raws = append(raws, raw)
		if err != nil && strings.Contains(raw.Endpoint, "/carriers/"+dot) && !strings.Contains(raw.Endpoint, "/authority") && !strings.Contains(raw.Endpoint, "/basics") && !strings.Contains(raw.Endpoint, "/docket-numbers") && !strings.Contains(raw.Endpoint, "/oos") {
			return domain.LookupResult{}, mapSourceErr(err)
		}
	}
	return a.resultFromRaw(ctx, req, raws, "live")
}

func (a *App) lookupFixture(ctx context.Context, req domain.LookupRequest) (domain.LookupResult, error) {
	body, err := os.ReadFile(req.FixturePath)
	if err != nil {
		return domain.LookupResult{}, err
	}
	var existing domain.LookupResult
	if json.Unmarshal(body, &existing) == nil && existing.Carrier.USDOTNumber != "" {
		return a.persistPreparedResult(ctx, req, existing, "fixture", []fmcsa.RawResponse{{
			Endpoint: "fixture:" + req.FixturePath,
			Body:     body,
			Fetch: domain.SourceFetchResult{
				SourceName:         "fixture",
				Endpoint:           req.FixturePath,
				RequestMethod:      "GET",
				RequestURLRedacted: req.FixturePath,
				FetchedAt:          time.Now().UTC().Format(time.RFC3339),
				ResponseHash:       normalize.HashRaw(body),
			},
		}})
	}
	raw := fmcsa.RawResponse{
		Endpoint: "fixture:" + filepath.Base(req.FixturePath),
		Body:     body,
		Fetch: domain.SourceFetchResult{
			SourceName:         "fixture",
			Endpoint:           req.FixturePath,
			RequestMethod:      "GET",
			RequestURLRedacted: req.FixturePath,
			FetchedAt:          time.Now().UTC().Format(time.RFC3339),
			ResponseHash:       normalize.HashRaw(body),
		},
	}
	return a.resultFromRaw(ctx, req, []fmcsa.RawResponse{raw}, "fixture")
}

func (a *App) lookupOffline(ctx context.Context, req domain.LookupRequest) (domain.LookupResult, error) {
	carrier, obs, err := a.Store.LatestCarrierByIdentifier(ctx, req.IdentifierType, req.IdentifierValue)
	if err != nil {
		if store.IsNotFound(err) || errors.Is(err, sql.ErrNoRows) {
			return domain.LookupResult{}, apperrors.New(apperrors.CodeOfflineCacheMiss, "this carrier is not in the local cache", "Run without --offline after completing setup")
		}
		return domain.LookupResult{}, err
	}
	count, _ := a.Store.ObservationCount(ctx, carrier.USDOTNumber)
	assessment := scoring.Assess(carrier, scoring.Context{ObservationCount: count, ObservedAt: time.Now().UTC()})
	return buildLookupResult(req, carrier, assessment, nil, "offline", obs.ObservedAt), nil
}

func (a *App) lookupFreshCache(ctx context.Context, req domain.LookupRequest) (domain.LookupResult, bool) {
	carrier, obs, err := a.Store.LatestCarrierByIdentifier(ctx, req.IdentifierType, req.IdentifierValue)
	if err != nil {
		return domain.LookupResult{}, false
	}
	observedAt, err := time.Parse(time.RFC3339, obs.ObservedAt)
	if err != nil || time.Since(observedAt) > req.MaxAge {
		return domain.LookupResult{}, false
	}
	count, _ := a.Store.ObservationCount(ctx, carrier.USDOTNumber)
	assessment := scoring.Assess(carrier, scoring.Context{ObservationCount: count, ObservedAt: time.Now().UTC()})
	return buildLookupResult(req, carrier, assessment, nil, "cache", obs.ObservedAt), true
}

func (a *App) resultFromRaw(ctx context.Context, req domain.LookupRequest, raws []fmcsa.RawResponse, mode string) (domain.LookupResult, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	carrier, err := normalize.FMCSAResponsesToCarrier(req.IdentifierType, req.IdentifierValue, raws, now)
	if err != nil {
		return domain.LookupResult{}, err
	}
	if previous, _, err := a.Store.LatestCarrierByUSDOT(ctx, carrier.USDOTNumber); err == nil && previous.LocalFirstSeenAt != "" {
		carrier.LocalFirstSeenAt = previous.LocalFirstSeenAt
	}
	count, _ := a.Store.ObservationCount(ctx, carrier.USDOTNumber)
	assessment := scoring.Assess(carrier, scoring.Context{ObservationCount: count + 1, ObservedAt: time.Now().UTC()})
	result := buildLookupResult(req, carrier, assessment, fetches(raws), mode, now)
	return a.persistPreparedResult(ctx, req, result, mode, raws)
}

func (a *App) persistPreparedResult(ctx context.Context, req domain.LookupRequest, result domain.LookupResult, mode string, raws []fmcsa.RawResponse) (domain.LookupResult, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if result.SchemaVersion == "" {
		result.SchemaVersion = domain.SchemaVersion
	}
	if result.ReportType == "" {
		result.ReportType = "carrier_lookup_report"
	}
	if result.GeneratedAt == "" {
		result.GeneratedAt = now
	}
	result.Disclaimer = domain.Disclaimer
	result.Lookup.InputType = req.IdentifierType
	result.Lookup.InputValue = req.IdentifierValue
	result.Lookup.ResolvedUSDOT = result.Carrier.USDOTNumber
	result.Lookup.LookedUpAt = result.GeneratedAt
	result.Lookup.Mode = mode
	result.Lookup.LocalFirstSeen = result.Carrier.LocalFirstSeenAt
	if len(result.Freshness.Sources) == 0 {
		result.Freshness = freshnessFromFetches(mode, result.Sources)
	}
	rawGroupID := fmt.Sprintf("%s_%d", result.Carrier.USDOTNumber, time.Now().UnixNano())
	for i := range raws {
		path, err := a.writeRaw(rawGroupID, result.Carrier.USDOTNumber, raws[i])
		if err == nil {
			raws[i].Fetch.RawPath = path
		}
	}
	result.Sources = fetches(raws)
	hash, err := normalize.HashNormalized(result.Carrier)
	if err != nil {
		return domain.LookupResult{}, err
	}
	_, assessmentID, err := a.Store.SaveLookup(ctx, result, hash, rawGroupID)
	if err != nil {
		return domain.LookupResult{}, apperrors.Wrap(apperrors.CodeDatabase, "could not save carrier lookup snapshot", "Run: ohg doctor", err)
	}
	result.RiskAssessment.ID = assessmentID
	return result, nil
}

func buildLookupResult(req domain.LookupRequest, carrier domain.CarrierProfile, assessment domain.RiskAssessment, sources []domain.SourceFetchResult, mode, observedAt string) domain.LookupResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if observedAt != "" {
		now = observedAt
	}
	return domain.LookupResult{
		SchemaVersion: domain.SchemaVersion,
		ReportType:    "carrier_lookup_report",
		GeneratedAt:   now,
		Lookup: domain.LookupInfo{
			InputType:      req.IdentifierType,
			InputValue:     req.IdentifierValue,
			ResolvedUSDOT:  carrier.USDOTNumber,
			LookedUpAt:     now,
			Mode:           mode,
			LocalFirstSeen: carrier.LocalFirstSeenAt,
		},
		Carrier:        carrier,
		Freshness:      freshnessFromFetches(mode, sources),
		RiskAssessment: assessment,
		Sources:        sources,
		Disclaimer:     domain.Disclaimer,
	}
}

func (a *App) writeRaw(rawGroupID, usdot string, raw fmcsa.RawResponse) (string, error) {
	if len(raw.Body) == 0 {
		return "", nil
	}
	now := time.Now().UTC()
	hash := normalize.HashRaw(raw.Body)
	dir := filepath.Join(a.Config.RawDir, raw.Fetch.SourceName, "carrier_"+usdot, now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s_%s.json", now.Format("20060102T150405Z"), hash[:12])
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, raw.Body, 0o600)
}

func (a *App) Diff(ctx context.Context, typ, value, since string, strict bool) (domain.DiffResult, error) {
	typ, value, err := normalize.Identifier(typ, value)
	if err != nil {
		return domain.DiffResult{}, err
	}
	sinceTime, err := parseSince(since)
	if err != nil {
		return domain.DiffResult{}, err
	}
	observations, err := a.Store.ObservationsSince(ctx, typ, value, sinceTime)
	if err != nil {
		return domain.DiffResult{}, err
	}
	result := domain.DiffResult{
		SchemaVersion:    domain.SchemaVersion,
		ReportType:       "carrier_diff_report",
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		IdentifierType:   typ,
		IdentifierValue:  value,
		ObservationCount: len(observations),
	}
	if len(observations) == 0 {
		return result, nil
	}
	result.ResolvedUSDOT = observations[len(observations)-1].USDOTNumber
	if len(observations) < 2 {
		return result, nil
	}
	var previous, current domain.CarrierProfile
	if err := json.Unmarshal([]byte(observations[0].NormalizedJSON), &previous); err != nil {
		return result, err
	}
	if err := json.Unmarshal([]byte(observations[len(observations)-1].NormalizedJSON), &current); err != nil {
		return result, err
	}
	fields := map[string][2]string{
		"legal_name":       {previous.LegalName, current.LegalName},
		"dba_name":         {previous.DBAName, current.DBAName},
		"physical_address": {joinAddress(previous.PhysicalAddress), joinAddress(current.PhysicalAddress)},
		"mailing_address":  {joinAddress(previous.MailingAddress), joinAddress(current.MailingAddress)},
		"phone":            {previous.Contact.Phone, current.Contact.Phone},
		"email":            {previous.Contact.Email, current.Contact.Email},
		"authority.status": {joinAuthority(previous.Authority), joinAuthority(current.Authority)},
		"power_units":      {strconv.Itoa(previous.Operations.PowerUnits), strconv.Itoa(current.Operations.PowerUnits)},
		"drivers":          {strconv.Itoa(previous.Operations.Drivers), strconv.Itoa(current.Operations.Drivers)},
	}
	keys := []string{"legal_name", "dba_name", "physical_address", "mailing_address", "phone", "email", "authority.status", "power_units", "drivers"}
	for _, key := range keys {
		values := fields[key]
		left, right := values[0], values[1]
		compareLeft, compareRight := left, right
		if !strict {
			compareLeft = normalize.ComparableString(left)
			compareRight = normalize.ComparableString(right)
		}
		if compareLeft != compareRight {
			result.Changes = append(result.Changes, domain.FieldDiff{
				FieldPath:          key,
				PreviousValue:      left,
				CurrentValue:       right,
				PreviousObservedAt: observations[0].ObservedAt,
				CurrentObservedAt:  observations[len(observations)-1].ObservedAt,
				Material:           true,
			})
		}
	}
	return result, nil
}

func (a *App) WatchAdd(ctx context.Context, typ, value, label string) error {
	typ, value, err := normalize.Identifier(typ, value)
	if err != nil {
		return err
	}
	usdot := ""
	if resolved, err := a.Store.ResolveIdentifier(ctx, typ, value); err == nil {
		usdot = resolved
	}
	return a.Store.AddWatch(ctx, typ, value, label, usdot)
}

func (a *App) WatchList(ctx context.Context) ([]store.WatchEntry, error) {
	return a.Store.ListWatch(ctx)
}

type WatchSyncResult struct {
	SchemaVersion string               `json:"schema_version"`
	GeneratedAt   string               `json:"generated_at"`
	Synced        int                  `json:"synced"`
	Failed        int                  `json:"failed"`
	Results       []domain.LookupInfo  `json:"results"`
	Warnings      []domain.UserWarning `json:"warnings,omitempty"`
}

func (a *App) WatchSync(ctx context.Context, fixture string, force bool) (WatchSyncResult, error) {
	items, err := a.Store.ListWatch(ctx)
	if err != nil {
		return WatchSyncResult{}, err
	}
	out := WatchSyncResult{SchemaVersion: domain.SchemaVersion, GeneratedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, item := range items {
		result, err := a.Lookup(ctx, domain.LookupRequest{
			IdentifierType:  item.IdentifierType,
			IdentifierValue: item.IdentifierValue,
			ForceRefresh:    force,
			FixturePath:     fixture,
		})
		if err != nil {
			out.Failed++
			out.Warnings = append(out.Warnings, domain.UserWarning{Code: "OHG_WATCH_SYNC_FAILED", Message: err.Error()})
			continue
		}
		out.Synced++
		out.Results = append(out.Results, result.Lookup)
		_ = a.Store.MarkWatchSynced(ctx, item.ID, result.Carrier.USDOTNumber)
	}
	return out, nil
}

func fetches(raws []fmcsa.RawResponse) []domain.SourceFetchResult {
	out := make([]domain.SourceFetchResult, 0, len(raws))
	for _, raw := range raws {
		out = append(out, raw.Fetch)
	}
	return out
}

func freshnessFromFetches(mode string, sources []domain.SourceFetchResult) domain.FreshnessSummary {
	out := domain.FreshnessSummary{Mode: mode}
	for _, source := range sources {
		out.Sources = append(out.Sources, domain.FreshnessItem{
			Source:    source.SourceName,
			FetchedAt: source.FetchedAt,
			Notes:     source.Endpoint,
		})
	}
	if len(out.Sources) == 0 {
		out.Sources = append(out.Sources, domain.FreshnessItem{Source: "local_cache", Notes: "No network source was called for this report."})
	}
	return out
}

func extractDOTFromRaw(body []byte) string {
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	carrier, err := normalize.FMCSAResponsesToCarrier("dot", "", []fmcsa.RawResponse{{Endpoint: "fixture", Body: body}}, time.Now().UTC().Format(time.RFC3339))
	if err == nil {
		return carrier.USDOTNumber
	}
	return ""
}

func mapSourceErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "WebKey"):
		return apperrors.Wrap(apperrors.CodeAuthFMCSAInvalid, "FMCSA rejected the configured WebKey", "Run: ohg setup fmcsa --reset", err)
	case strings.Contains(msg, "not found"):
		return apperrors.Wrap(apperrors.CodeSourceNotFound, "carrier was not found in the source", "", err)
	case strings.Contains(msg, "rate"):
		return apperrors.Wrap(apperrors.CodeSourceRateLimited, "source rate-limited the request", "Try again later", err)
	default:
		return apperrors.Wrap(apperrors.CodeSourceUnavailable, "source lookup failed", "Try again later or use --offline if cached", err)
	}
}

func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().UTC().Add(-90 * 24 * time.Hour), nil
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().UTC().Add(-d), nil
	}
	return time.Parse("2006-01-02", s)
}

func joinAddress(a domain.Address) string {
	parts := []string{a.Line1, a.Line2, a.City, a.State, a.PostalCode, a.Country}
	var out []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return strings.Join(out, ", ")
}

func joinAuthority(records []domain.AuthorityRecord) string {
	var out []string
	for _, r := range records {
		value := strings.TrimSpace(r.AuthorityType + " " + r.AuthorityStatus)
		if value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, ", ")
}

func openURL(raw string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", raw)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw)
	default:
		cmd = exec.Command("xdg-open", raw)
	}
	return cmd.Start()
}
