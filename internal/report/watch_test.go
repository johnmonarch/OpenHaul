package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openhaulguard/openhaulguard/internal/app"
)

func TestWriteWatchExportFormats(t *testing.T) {
	result := app.WatchExportResult{
		GeneratedAt: "2026-04-25T12:00:00Z",
		Total:       1,
		Items: []app.WatchExportItem{{
			IdentifierType:  "mc",
			IdentifierValue: "123456",
			NormalizedValue: "123456",
			USDOTNumber:     "1234567",
			Label:           "primary | lane",
			Active:          true,
			CreatedAt:       "2026-04-25T12:00:00Z",
			UpdatedAt:       "2026-04-25T12:01:00Z",
			LastSyncedAt:    "2026-04-25T12:02:00Z",
		}},
	}

	var jsonOut bytes.Buffer
	if err := WriteWatchExport(&jsonOut, result, "json"); err != nil {
		t.Fatalf("WriteWatchExport json failed: %v", err)
	}
	var decoded app.WatchExportResult
	if err := json.Unmarshal(jsonOut.Bytes(), &decoded); err != nil {
		t.Fatalf("json output did not decode: %v\n%s", err, jsonOut.String())
	}
	if decoded.Total != 1 || len(decoded.Items) != 1 || decoded.Items[0].IdentifierType != "mc" {
		t.Fatalf("decoded export = %#v", decoded)
	}

	var markdownOut bytes.Buffer
	if err := WriteWatchExport(&markdownOut, result, "markdown"); err != nil {
		t.Fatalf("WriteWatchExport markdown failed: %v", err)
	}
	markdown := markdownOut.String()
	if !strings.Contains(markdown, "# OpenHaul Guard Watchlist Export") {
		t.Fatalf("markdown output missing heading: %s", markdown)
	}
	if !strings.Contains(markdown, "primary \\| lane") {
		t.Fatalf("markdown output did not escape label: %s", markdown)
	}

	var tableOut bytes.Buffer
	if err := WriteWatchExport(&tableOut, result, "table"); err != nil {
		t.Fatalf("WriteWatchExport table failed: %v", err)
	}
	table := tableOut.String()
	if !strings.Contains(table, "OpenHaul Guard watchlist export") || !strings.Contains(table, "MC 123456") || !strings.Contains(table, "active=true") {
		t.Fatalf("table output missing expected fields: %s", table)
	}
}
