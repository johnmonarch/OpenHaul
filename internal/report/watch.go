package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/openhaulguard/openhaulguard/internal/app"
)

func WriteWatch(w io.Writer, result app.WatchReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "markdown", "md":
		_, err := fmt.Fprint(w, WatchMarkdown(result))
		return err
	case "table", "":
		_, err := fmt.Fprint(w, WatchTable(result))
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func WriteWatchExport(w io.Writer, result app.WatchExportResult, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "markdown", "md":
		_, err := fmt.Fprint(w, WatchExportMarkdown(result))
		return err
	case "table", "":
		_, err := fmt.Fprint(w, WatchExportTable(result))
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func WatchExportMarkdown(result app.WatchExportResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# OpenHaul Guard Watchlist Export\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", result.GeneratedAt)
	fmt.Fprintf(&b, "Total: %d\n\n", result.Total)
	fmt.Fprintf(&b, "| Identifier | Normalized | USDOT | Label | Active | Created | Updated | Last synced |\n|---|---|---|---|---|---|---|---|\n")
	for _, item := range result.Items {
		fmt.Fprintf(&b, "| %s %s | %s | %s | %s | %t | %s | %s | %s |\n",
			strings.ToUpper(item.IdentifierType),
			escape(item.IdentifierValue),
			escape(item.NormalizedValue),
			escape(item.USDOTNumber),
			escape(item.Label),
			item.Active,
			escape(item.CreatedAt),
			escape(item.UpdatedAt),
			escape(item.LastSyncedAt))
	}
	return b.String()
}

func WatchExportTable(result app.WatchExportResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "OpenHaul Guard watchlist export\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", result.GeneratedAt)
	fmt.Fprintf(&b, "Total: %d\n\n", result.Total)
	if len(result.Items) == 0 {
		fmt.Fprintf(&b, "No active watchlist entries.\n")
		return b.String()
	}
	for _, item := range result.Items {
		fmt.Fprintf(&b, "%s %s", strings.ToUpper(item.IdentifierType), item.IdentifierValue)
		if item.NormalizedValue != "" && item.NormalizedValue != item.IdentifierValue {
			fmt.Fprintf(&b, " normalized %s", item.NormalizedValue)
		}
		if item.USDOTNumber != "" {
			fmt.Fprintf(&b, " USDOT %s", item.USDOTNumber)
		}
		if item.Label != "" {
			fmt.Fprintf(&b, " [%s]", item.Label)
		}
		fmt.Fprintf(&b, " active=%t", item.Active)
		if item.CreatedAt != "" {
			fmt.Fprintf(&b, " created=%s", item.CreatedAt)
		}
		if item.UpdatedAt != "" {
			fmt.Fprintf(&b, " updated=%s", item.UpdatedAt)
		}
		if item.LastSyncedAt != "" {
			fmt.Fprintf(&b, " last_synced=%s", item.LastSyncedAt)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func WatchMarkdown(result app.WatchReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# OpenHaul Guard Watch Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", result.GeneratedAt)
	fmt.Fprintf(&b, "Since: %s\n", result.Since)
	if result.Label != "" {
		fmt.Fprintf(&b, "Label filter: %s\n", result.Label)
	}
	fmt.Fprintf(&b, "\nSummary: %d watched, %d changed, %d unchanged, %d no data.\n\n", result.Total, result.Changed, result.Unchanged, result.NoData)
	fmt.Fprintf(&b, "| Watch | Label | USDOT | Status | Changes | Last observed |\n|---|---|---|---|---|---|\n")
	for _, item := range result.Items {
		fmt.Fprintf(&b, "| %s %s | %s | %s | %s | %d | %s |\n",
			strings.ToUpper(item.IdentifierType),
			escape(item.IdentifierValue),
			escape(item.Label),
			escape(item.USDOTNumber),
			escape(item.Status),
			len(item.Changes),
			escape(item.LastObservedAt))
	}
	for _, item := range result.Items {
		if len(item.Changes) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s %s\n\n", strings.ToUpper(item.IdentifierType), item.IdentifierValue)
		fmt.Fprintf(&b, "| Field | Previous | Current |\n|---|---|---|\n")
		for _, change := range item.Changes {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", escape(change.FieldPath), escape(change.PreviousValue), escape(change.CurrentValue))
		}
	}
	return b.String()
}

func WatchTable(result app.WatchReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "OpenHaul Guard watch report\n\n")
	fmt.Fprintf(&b, "Since: %s\n", result.Since)
	if result.Label != "" {
		fmt.Fprintf(&b, "Label: %s\n", result.Label)
	}
	fmt.Fprintf(&b, "Summary: %d watched, %d changed, %d unchanged, %d no data\n\n", result.Total, result.Changed, result.Unchanged, result.NoData)
	if len(result.Items) == 0 {
		fmt.Fprintf(&b, "No watched carriers matched.\n")
		return b.String()
	}
	for _, item := range result.Items {
		fmt.Fprintf(&b, "%s %s", strings.ToUpper(item.IdentifierType), item.IdentifierValue)
		if item.Label != "" {
			fmt.Fprintf(&b, " [%s]", item.Label)
		}
		if item.USDOTNumber != "" {
			fmt.Fprintf(&b, " USDOT %s", item.USDOTNumber)
		}
		fmt.Fprintf(&b, ": %s", item.Status)
		if len(item.Changes) > 0 {
			var fields []string
			for _, change := range item.Changes {
				fields = append(fields, change.FieldPath)
			}
			fmt.Fprintf(&b, " (%s)", strings.Join(fields, ", "))
		}
		if item.Error != "" {
			fmt.Fprintf(&b, " - %s", item.Error)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}
