package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/normalize"
	"github.com/openhaulguard/openhaulguard/internal/store"
)

type WatchReport struct {
	SchemaVersion string            `json:"schema_version"`
	ReportType    string            `json:"report_type"`
	GeneratedAt   string            `json:"generated_at"`
	Since         string            `json:"since"`
	Label         string            `json:"label,omitempty"`
	Total         int               `json:"total"`
	Changed       int               `json:"changed"`
	Unchanged     int               `json:"unchanged"`
	NoData        int               `json:"no_data"`
	Items         []WatchReportItem `json:"items"`
}

type WatchReportItem struct {
	WatchID          int64              `json:"watch_id"`
	IdentifierType   string             `json:"identifier_type"`
	IdentifierValue  string             `json:"identifier_value"`
	Label            string             `json:"label,omitempty"`
	USDOTNumber      string             `json:"usdot_number,omitempty"`
	Status           string             `json:"status"`
	ObservationCount int                `json:"observation_count"`
	Changes          []domain.FieldDiff `json:"changes,omitempty"`
	LastObservedAt   string             `json:"last_observed_at,omitempty"`
	Error            string             `json:"error,omitempty"`
}

type WatchExportResult struct {
	SchemaVersion string            `json:"schema_version"`
	ReportType    string            `json:"report_type"`
	GeneratedAt   string            `json:"generated_at"`
	Total         int               `json:"total"`
	Items         []WatchExportItem `json:"items"`
}

type WatchExport = WatchExportResult

type WatchExportItem struct {
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

func (a *App) WatchRemove(ctx context.Context, typ, value string) (bool, error) {
	typ, value, err := normalize.Identifier(typ, value)
	if err != nil {
		return false, err
	}
	return a.Store.RemoveWatch(ctx, typ, value)
}

func (a *App) WatchExport(ctx context.Context) (WatchExportResult, error) {
	items, err := a.Store.ExportWatch(ctx)
	if err != nil {
		return WatchExportResult{}, err
	}
	out := WatchExportResult{
		SchemaVersion: domain.SchemaVersion,
		ReportType:    "watchlist_export_report",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Total:         len(items),
	}
	for _, item := range items {
		out.Items = append(out.Items, WatchExportItem{
			IdentifierType:  item.IdentifierType,
			IdentifierValue: item.IdentifierValue,
			NormalizedValue: item.NormalizedValue,
			USDOTNumber:     item.USDOTNumber,
			Label:           item.Label,
			Active:          item.Active,
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
			LastSyncedAt:    item.LastSyncedAt,
		})
	}
	return out, nil
}

func (a *App) WatchReport(ctx context.Context, since, label string) (WatchReport, error) {
	sinceTime, err := parseSince(since)
	if err != nil {
		return WatchReport{}, apperrors.Wrap(apperrors.CodeInvalidArgs, "invalid --since value", "Use a duration like 24h or a date like 2026-04-25", err)
	}
	items, err := a.Store.ListWatch(ctx)
	if err != nil {
		return WatchReport{}, err
	}
	out := WatchReport{
		SchemaVersion: domain.SchemaVersion,
		ReportType:    "watch_report",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Since:         sinceTime.UTC().Format(time.RFC3339),
		Label:         strings.TrimSpace(label),
	}
	for _, item := range items {
		if !watchLabelMatches(item, label) {
			continue
		}
		reportItem := a.watchReportItem(ctx, item, sinceTime)
		out.Total++
		switch reportItem.Status {
		case "changed":
			out.Changed++
		case "no_data":
			out.NoData++
		default:
			out.Unchanged++
		}
		out.Items = append(out.Items, reportItem)
	}
	return out, nil
}

func (a *App) watchReportItem(ctx context.Context, item store.WatchEntry, sinceTime time.Time) WatchReportItem {
	out := WatchReportItem{
		WatchID:         item.ID,
		IdentifierType:  item.IdentifierType,
		IdentifierValue: item.IdentifierValue,
		Label:           item.Label,
		USDOTNumber:     item.USDOTNumber,
		Status:          "unchanged",
	}
	observations, err := a.Store.ObservationsSince(ctx, item.IdentifierType, item.NormalizedValue, sinceTime)
	if err != nil {
		out.Status = "no_data"
		out.Error = err.Error()
		return out
	}
	out.ObservationCount = len(observations)
	if len(observations) == 0 {
		out.Status = "no_data"
		return out
	}
	out.USDOTNumber = observations[len(observations)-1].USDOTNumber
	out.LastObservedAt = observations[len(observations)-1].ObservedAt
	if len(observations) < 2 {
		return out
	}
	var previous, current domain.CarrierProfile
	if err := json.Unmarshal([]byte(observations[0].NormalizedJSON), &previous); err != nil {
		out.Status = "no_data"
		out.Error = err.Error()
		return out
	}
	if err := json.Unmarshal([]byte(observations[len(observations)-1].NormalizedJSON), &current); err != nil {
		out.Status = "no_data"
		out.Error = err.Error()
		return out
	}
	out.Changes = carrierFieldDiffs(previous, current, observations[0].ObservedAt, observations[len(observations)-1].ObservedAt, false)
	if len(out.Changes) > 0 {
		out.Status = "changed"
	}
	return out
}

func watchLabelMatches(item store.WatchEntry, label string) bool {
	label = strings.TrimSpace(label)
	if label == "" {
		return true
	}
	return strings.Contains(strings.ToLower(item.Label), strings.ToLower(label))
}
