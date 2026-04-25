package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/store"
)

func TestWatchExportIncludesActiveEntries(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	svc := &App{Store: st}
	defer svc.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.AddWatch(ctx, "mc", "123456", "primary lane", "1234567"); err != nil {
		t.Fatal(err)
	}
	active, err := st.ListWatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("active watch entries = %d, want 1", len(active))
	}
	if err := st.MarkWatchSynced(ctx, active[0].ID, "1234567"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddWatch(ctx, "dot", "7654321", "inactive lane", "7654321"); err != nil {
		t.Fatal(err)
	}
	removed, err := st.RemoveWatch(ctx, "dot", "7654321")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("RemoveWatch returned false, want true")
	}

	result, err := svc.WatchExport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != domain.SchemaVersion {
		t.Fatalf("schema version = %q, want %q", result.SchemaVersion, domain.SchemaVersion)
	}
	if result.ReportType != "watchlist_export_report" {
		t.Fatalf("report type = %q, want watchlist_export_report", result.ReportType)
	}
	if _, err := time.Parse(time.RFC3339, result.GeneratedAt); err != nil {
		t.Fatalf("generated_at did not parse as RFC3339: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("export summary = total %d len %d, want 1/1", result.Total, len(result.Items))
	}
	item := result.Items[0]
	if item.IdentifierType != "mc" || item.IdentifierValue != "123456" || item.NormalizedValue != "123456" {
		t.Fatalf("identifier fields = %#v", item)
	}
	if item.USDOTNumber != "1234567" || item.Label != "primary lane" || !item.Active {
		t.Fatalf("export item = %#v", item)
	}
	if item.CreatedAt == "" || item.UpdatedAt == "" || item.LastSyncedAt == "" {
		t.Fatalf("expected timestamps to be populated: %#v", item)
	}
}
