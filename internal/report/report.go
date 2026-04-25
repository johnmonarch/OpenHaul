package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/openhaulguard/openhaulguard/internal/domain"
)

func WriteLookup(w io.Writer, result domain.LookupResult, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "markdown", "md":
		_, err := fmt.Fprint(w, LookupMarkdown(result))
		return err
	case "table", "":
		_, err := fmt.Fprint(w, LookupTable(result))
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func LookupMarkdown(result domain.LookupResult) string {
	var b strings.Builder
	c := result.Carrier
	fmt.Fprintf(&b, "# OpenHaul Guard Carrier Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", result.GeneratedAt)
	fmt.Fprintf(&b, "Lookup input: %s %s\n", result.Lookup.InputType, result.Lookup.InputValue)
	fmt.Fprintf(&b, "Resolved USDOT: %s\n", result.Lookup.ResolvedUSDOT)
	fmt.Fprintf(&b, "Mode: %s\n\n", result.Lookup.Mode)
	fmt.Fprintf(&b, "## Carrier\n\n")
	fmt.Fprintf(&b, "| Field | Value |\n|---|---|\n")
	row(&b, "Legal name", c.LegalName)
	row(&b, "DBA", c.DBAName)
	row(&b, "USDOT", c.USDOTNumber)
	row(&b, "Docket numbers", identifiers(c.Identifiers))
	row(&b, "Authority status", authorityStatus(c.Authority))
	row(&b, "Physical address", address(c.PhysicalAddress))
	row(&b, "Phone", c.Contact.Phone)
	row(&b, "Power units", fmt.Sprint(c.Operations.PowerUnits))
	row(&b, "Drivers", fmt.Sprint(c.Operations.Drivers))
	fmt.Fprintf(&b, "\n## Data Freshness\n\n")
	fmt.Fprintf(&b, "| Source | Freshness | Notes |\n|---|---|---|\n")
	for _, item := range result.Freshness.Sources {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", escape(item.Source), escape(item.FetchedAt), escape(item.Notes))
	}
	fmt.Fprintf(&b, "\n## Risk Review\n\n")
	fmt.Fprintf(&b, "Recommendation: %s\n", result.RiskAssessment.Recommendation)
	fmt.Fprintf(&b, "Score: %d/100\n\n", result.RiskAssessment.Score)
	fmt.Fprintf(&b, "### Flags\n\n")
	if len(result.RiskAssessment.Flags) == 0 {
		fmt.Fprintf(&b, "No obvious issue flags were triggered.\n\n")
	}
	for _, flag := range result.RiskAssessment.Flags {
		fmt.Fprintf(&b, "### %s: %s\n\n", flag.Severity, flag.Code)
		fmt.Fprintf(&b, "What we found: %s\n", flag.Explanation)
		fmt.Fprintf(&b, "Why it matters: %s\n", flag.WhyItMatters)
		fmt.Fprintf(&b, "Suggested next step: %s\n\n", flag.NextStep)
		fmt.Fprintf(&b, "Evidence:\n\n")
		fmt.Fprintf(&b, "| Field | Value | Source | Observed |\n|---|---|---|---|\n")
		for _, ev := range flag.Evidence {
			fmt.Fprintf(&b, "| %s | %v | %s | %s |\n", escape(ev.Field), ev.Value, escape(ev.Source), escape(ev.ObservedAt))
		}
		fmt.Fprintf(&b, "\n")
	}
	fmt.Fprintf(&b, "## Source Facts vs Inferences\n\n")
	fmt.Fprintf(&b, "Source facts are values returned by public sources. Risk flags are OpenHaul Guard interpretations intended for manual review.\n\n")
	fmt.Fprintf(&b, "## Disclaimer\n\n%s\n", domain.Disclaimer)
	return b.String()
}

func LookupTable(result domain.LookupResult) string {
	counts := map[string]int{}
	highest := "none"
	order := map[string]int{"none": 0, "info": 1, "low": 2, "medium": 3, "high": 4, "critical": 5}
	for _, flag := range result.RiskAssessment.Flags {
		counts[flag.Severity]++
		if order[flag.Severity] > order[highest] {
			highest = flag.Severity
		}
	}
	var severities []string
	for sev, count := range counts {
		severities = append(severities, fmt.Sprintf("%s=%d", sev, count))
	}
	sort.Strings(severities)
	var b strings.Builder
	fmt.Fprintf(&b, "OpenHaul Guard carrier lookup\n\n")
	fmt.Fprintf(&b, "Carrier: %s\n", blank(result.Carrier.LegalName))
	fmt.Fprintf(&b, "USDOT: %s\n", blank(result.Carrier.USDOTNumber))
	fmt.Fprintf(&b, "Identifiers: %s\n", identifiers(result.Carrier.Identifiers))
	fmt.Fprintf(&b, "Authority: %s\n", blank(authorityStatus(result.Carrier.Authority)))
	fmt.Fprintf(&b, "Mode: %s\n", result.Lookup.Mode)
	fmt.Fprintf(&b, "Recommendation: %s\n", result.RiskAssessment.Recommendation)
	fmt.Fprintf(&b, "Highest severity: %s\n", highest)
	fmt.Fprintf(&b, "Flag counts: %s\n", strings.Join(severities, ", "))
	fmt.Fprintf(&b, "\nUse --format markdown or --format json for evidence details.\n")
	return b.String()
}

func WriteDiff(w io.Writer, result domain.DiffResult, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "markdown", "md":
		var b strings.Builder
		fmt.Fprintf(&b, "# OpenHaul Guard Carrier Diff\n\n")
		fmt.Fprintf(&b, "Generated: %s\n", result.GeneratedAt)
		fmt.Fprintf(&b, "Lookup input: %s %s\n", result.IdentifierType, result.IdentifierValue)
		fmt.Fprintf(&b, "Resolved USDOT: %s\n", result.ResolvedUSDOT)
		fmt.Fprintf(&b, "Observation count: %d\n\n", result.ObservationCount)
		fmt.Fprintf(&b, "| Field | Previous | Current | Previous observed | Current observed |\n|---|---|---|---|---|\n")
		for _, change := range result.Changes {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", escape(change.FieldPath), escape(change.PreviousValue), escape(change.CurrentValue), escape(change.PreviousObservedAt), escape(change.CurrentObservedAt))
		}
		_, err := fmt.Fprint(w, b.String())
		return err
	case "table", "":
		var b strings.Builder
		fmt.Fprintf(&b, "OpenHaul Guard carrier diff\n\n")
		for _, change := range result.Changes {
			fmt.Fprintf(&b, "%s: %s -> %s\n", change.FieldPath, change.PreviousValue, change.CurrentValue)
		}
		if len(result.Changes) == 0 {
			fmt.Fprintf(&b, "No material changes found.\n")
		}
		_, err := fmt.Fprint(w, b.String())
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func row(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "| %s | %s |\n", escape(key), escape(blank(value)))
}

func identifiers(ids []domain.Identifier) string {
	if len(ids) == 0 {
		return ""
	}
	var out []string
	for _, id := range ids {
		out = append(out, strings.ToUpper(id.Type)+" "+id.Value)
	}
	return strings.Join(out, ", ")
}

func authorityStatus(records []domain.AuthorityRecord) string {
	if len(records) == 0 {
		return ""
	}
	var out []string
	for _, record := range records {
		value := strings.TrimSpace(record.AuthorityType + " " + record.AuthorityStatus)
		if value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, ", ")
}

func address(a domain.Address) string {
	parts := []string{a.Line1, a.Line2, a.City, a.State, a.PostalCode, a.Country}
	var out []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return strings.Join(out, ", ")
}

func blank(s string) string {
	if strings.TrimSpace(s) == "" || s == "0" {
		return "unknown"
	}
	return s
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
