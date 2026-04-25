package mirror

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/normalize"
)

const SourceName = "openhaul_mirror"

type Index struct {
	SchemaVersion   string                  `json:"schema_version"`
	GeneratedAt     string                  `json:"generated_at"`
	SourceTimestamp string                  `json:"source_timestamp,omitempty"`
	Attribution     string                  `json:"attribution,omitempty"`
	Carriers        []domain.CarrierProfile `json:"carriers"`
}

type Status struct {
	Available       bool   `json:"available"`
	Path            string `json:"path"`
	CarrierCount    int    `json:"carrier_count,omitempty"`
	GeneratedAt     string `json:"generated_at,omitempty"`
	SourceTimestamp string `json:"source_timestamp,omitempty"`
	Attribution     string `json:"attribution,omitempty"`
	Error           string `json:"error,omitempty"`
}

func Load(path string) (Index, []byte, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Index{}, nil, err
	}
	var index Index
	if err := json.Unmarshal(body, &index); err != nil {
		return Index{}, nil, err
	}
	return index, body, nil
}

func Inspect(path string) Status {
	status := Status{Path: path}
	index, _, err := Load(path)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Available = true
	status.CarrierCount = len(index.Carriers)
	status.GeneratedAt = index.GeneratedAt
	status.SourceTimestamp = index.SourceTimestamp
	status.Attribution = index.Attribution
	return status
}

func Lookup(_ context.Context, path, typ, value string, observedAt time.Time) (domain.CarrierProfile, domain.SourceFetchResult, bool, error) {
	index, body, err := Load(path)
	if err != nil {
		return domain.CarrierProfile{}, domain.SourceFetchResult{}, false, err
	}
	for _, carrier := range index.Carriers {
		if !matches(carrier, typ, value) {
			continue
		}
		now := observedAt.UTC().Format(time.RFC3339)
		if carrier.SourceFirstSeenAt == "" {
			carrier.SourceFirstSeenAt = firstNonEmpty(index.SourceTimestamp, index.GeneratedAt, now)
		}
		if carrier.LocalFirstSeenAt == "" {
			carrier.LocalFirstSeenAt = now
		}
		carrier.LocalLastSeenAt = now
		return carrier, domain.SourceFetchResult{
			SourceName:         SourceName,
			Endpoint:           path,
			RequestMethod:      "GET",
			RequestURLRedacted: path,
			FetchedAt:          now,
			ResponseHash:       normalize.HashRaw(body),
		}, true, nil
	}
	return domain.CarrierProfile{}, domain.SourceFetchResult{}, false, nil
}

func matches(carrier domain.CarrierProfile, typ, value string) bool {
	switch strings.ToLower(typ) {
	case "dot":
		return digitsOnly(carrier.USDOTNumber) == digitsOnly(value)
	case "mc", "mx", "ff":
		wantType := strings.ToUpper(typ)
		wantValue := digitsOnly(value)
		for _, id := range carrier.Identifiers {
			if strings.ToUpper(id.Type) == wantType && digitsOnly(id.Value) == wantValue {
				return true
			}
		}
	case "name":
		want := normalize.ComparableString(value)
		return normalize.ComparableString(carrier.LegalName) == want || normalize.ComparableString(carrier.DBAName) == want
	}
	return false
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
