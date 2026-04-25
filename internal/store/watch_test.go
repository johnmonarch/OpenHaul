package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestExportWatchReturnsActiveEntries(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.AddWatch(ctx, "mc", "123456", "primary lane", "1234567"); err != nil {
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

	items, err := st.ExportWatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("exported entries = %d, want 1", len(items))
	}
	if items[0].IdentifierType != "mc" || items[0].IdentifierValue != "123456" || !items[0].Active {
		t.Fatalf("exported item = %#v", items[0])
	}
}
