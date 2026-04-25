package packet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/normalize"
)

type ExtractedFields struct {
	LegalName   string              `json:"legal_name,omitempty"`
	DBAName     string              `json:"dba_name,omitempty"`
	USDOTNumber string              `json:"usdot_number,omitempty"`
	Identifiers []domain.Identifier `json:"identifiers,omitempty"`
	Address     domain.Address      `json:"address,omitempty"`
	RawAddress  string              `json:"raw_address,omitempty"`
	Contact     domain.Contact      `json:"contact,omitempty"`
}

type FieldComparison struct {
	Field       string  `json:"field"`
	PacketValue string  `json:"packet_value,omitempty"`
	SourceValue string  `json:"source_value,omitempty"`
	Status      string  `json:"status"`
	Method      string  `json:"method,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

type Summary struct {
	Matches        int    `json:"matches"`
	Mismatches     int    `json:"mismatches"`
	MissingPacket  int    `json:"missing_packet"`
	MissingSource  int    `json:"missing_source"`
	Recommendation string `json:"recommendation"`
}

type CheckResult struct {
	SchemaVersion string                `json:"schema_version"`
	ReportType    string                `json:"report_type"`
	GeneratedAt   string                `json:"generated_at"`
	PacketPath    string                `json:"packet_path"`
	Lookup        domain.LookupInfo     `json:"lookup"`
	Extracted     ExtractedFields       `json:"extracted"`
	Carrier       domain.CarrierProfile `json:"carrier"`
	Comparisons   []FieldComparison     `json:"comparisons"`
	Summary       Summary               `json:"summary"`
	Warnings      []domain.UserWarning  `json:"warnings,omitempty"`
	Disclaimer    string                `json:"disclaimer"`
}

type ExtractResult struct {
	SchemaVersion string          `json:"schema_version"`
	ReportType    string          `json:"report_type"`
	GeneratedAt   string          `json:"generated_at"`
	PacketPath    string          `json:"packet_path"`
	Extracted     ExtractedFields `json:"extracted"`
	Disclaimer    string          `json:"disclaimer"`
}

var (
	legalNameRE = regexp.MustCompile(`(?im)^\s*(?:legal\s+name|carrier\s+legal\s+name|legal\s+business\s+name)\s*[:\-]\s*(.+?)\s*$`)
	dbaRE       = regexp.MustCompile(`(?im)^\s*(?:dba|d/b/a|doing\s+business\s+as)\s*[:\-]\s*(.+?)\s*$`)
	usdotRE     = regexp.MustCompile(`(?im)\b(?:USDOT|U\.S\.?\s*DOT)\s*(?:number|no\.?|#)?\s*[:#\-]?\s*([0-9]{5,9})\b`)
	docketRE    = regexp.MustCompile(`(?im)\b(MC|MX|FF)\s*(?:number|no\.?|#)?\s*[:#\-]?\s*([0-9]{3,8})\b`)
	phoneRE     = regexp.MustCompile(`(?im)\b(?:phone|telephone|tel|dispatch\s+phone|contact\s+phone)\s*[:\-]?\s*(\+?1?[\s.(\-]*[0-9]{3}[\s.)\-]*[0-9]{3}[\s.\-]*[0-9]{4})\b`)
	emailRE     = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	addressRE   = regexp.MustCompile(`(?im)^\s*(?:physical\s+address|business\s+address|carrier\s+address|street\s+address|address)\s*[:\-]\s*(.+?)\s*$`)
	cityStateRE = regexp.MustCompile(`(?i)^\s*(.+?)\s*,\s*([^,]+?)\s*,\s*([A-Z]{2})\s+([0-9]{5}(?:-[0-9]{4})?)(?:\s*,\s*(?:US|USA|United States))?\s*$`)
)

func Check(ctx context.Context, packetPath string, lookup domain.LookupResult) (CheckResult, error) {
	extracted, err := Extract(ctx, packetPath)
	if err != nil {
		return CheckResult{}, err
	}
	comparisons := Compare(extracted, lookup.Carrier)
	return CheckResult{
		SchemaVersion: domain.SchemaVersion,
		ReportType:    "packet_check_report",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		PacketPath:    packetPath,
		Lookup:        lookup.Lookup,
		Extracted:     extracted,
		Carrier:       lookup.Carrier,
		Comparisons:   comparisons,
		Summary:       summarize(comparisons),
		Warnings:      lookup.Warnings,
		Disclaimer:    domain.Disclaimer,
	}, nil
}

func ExtractReport(ctx context.Context, packetPath string) (ExtractResult, error) {
	extracted, err := Extract(ctx, packetPath)
	if err != nil {
		return ExtractResult{}, err
	}
	return ExtractResult{
		SchemaVersion: domain.SchemaVersion,
		ReportType:    "packet_extract_report",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		PacketPath:    packetPath,
		Extracted:     extracted,
		Disclaimer:    domain.Disclaimer,
	}, nil
}

func Extract(ctx context.Context, packetPath string) (ExtractedFields, error) {
	text, err := ExtractText(ctx, packetPath)
	if err != nil {
		return ExtractedFields{}, err
	}
	return ExtractFields(text), nil
}

func ExtractText(ctx context.Context, packetPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(packetPath))
	switch ext {
	case ".txt", ".text", "":
		body, err := os.ReadFile(packetPath)
		if err != nil {
			return "", err
		}
		return string(body), nil
	case ".pdf":
		if _, err := exec.LookPath("pdftotext"); err != nil {
			return "", apperrors.New(apperrors.CodePacketParseFailed, "pdftotext is required to extract text from PDF packets", "Install poppler/pdftotext or provide a .txt packet")
		}
		cmd := exec.CommandContext(ctx, "pdftotext", "-layout", packetPath, "-")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return "", apperrors.New(apperrors.CodePacketParseFailed, "could not extract text from PDF packet: "+msg, "Confirm the PDF contains selectable text or provide a .txt packet")
		}
		if strings.TrimSpace(string(out)) == "" {
			return "", apperrors.New(apperrors.CodePacketParseFailed, "PDF packet did not contain extractable text", "Use a text-based PDF or provide a .txt packet")
		}
		return string(out), nil
	default:
		return "", apperrors.New(apperrors.CodePacketParseFailed, "unsupported packet file type", "Provide a .txt packet or a text-based .pdf packet")
	}
}

func ExtractFields(text string) ExtractedFields {
	out := ExtractedFields{
		LegalName:   cleanValue(firstSubmatch(legalNameRE, text)),
		DBAName:     cleanValue(firstSubmatch(dbaRE, text)),
		USDOTNumber: digitsOnly(firstSubmatch(usdotRE, text)),
	}
	for _, match := range docketRE.FindAllStringSubmatch(text, -1) {
		if len(match) != 3 {
			continue
		}
		out.Identifiers = appendIdentifier(out.Identifiers, strings.ToUpper(match[1]), digitsOnly(match[2]))
	}
	if phone := firstSubmatch(phoneRE, text); phone != "" {
		out.Contact.Phone = normalize.Phone(phone)
	}
	if email := emailRE.FindString(text); email != "" {
		out.Contact.Email = strings.ToLower(strings.TrimSpace(email))
	}
	out.RawAddress = extractAddress(text)
	out.Address = parseAddress(out.RawAddress)
	return out
}

func Compare(packetFields ExtractedFields, carrier domain.CarrierProfile) []FieldComparison {
	var out []FieldComparison
	out = append(out, compareText("legal_name", packetFields.LegalName, carrier.LegalName, true))
	out = append(out, compareText("dba_name", packetFields.DBAName, carrier.DBAName, true))
	out = append(out, compareDigits("usdot_number", packetFields.USDOTNumber, carrier.USDOTNumber))
	out = append(out, compareAddress("physical_address", packetFields.RawAddress, joinAddress(carrier.PhysicalAddress)))
	out = append(out, comparePhone("phone", packetFields.Contact.Phone, carrier.Contact.Phone))
	out = append(out, compareEmail("email", packetFields.Contact.Email, carrier.Contact.Email))

	packetIDs := identifiersByType(packetFields.Identifiers)
	sourceIDs := identifiersByType(carrier.Identifiers)
	for _, typ := range []string{"MC", "MX", "FF"} {
		packetValue := packetIDs[typ]
		sourceValue := sourceIDs[typ]
		if packetValue == "" && sourceValue == "" {
			continue
		}
		out = append(out, compareDigits("identifier."+strings.ToLower(typ), packetValue, sourceValue))
	}
	return out
}

func Write(w io.Writer, result CheckResult, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "markdown", "md":
		_, err := fmt.Fprint(w, Markdown(result))
		return err
	case "table", "":
		_, err := fmt.Fprint(w, Table(result))
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func WriteExtract(w io.Writer, result ExtractResult, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "markdown", "md":
		_, err := fmt.Fprint(w, ExtractMarkdown(result))
		return err
	case "table", "":
		_, err := fmt.Fprint(w, ExtractTable(result))
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func ExtractMarkdown(result ExtractResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# OpenHaul Guard Packet Extract\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", result.GeneratedAt)
	fmt.Fprintf(&b, "Packet: %s\n\n", result.PacketPath)
	fmt.Fprintf(&b, "## Extracted Fields\n\n")
	fmt.Fprintf(&b, "| Field | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Legal name | %s |\n", escape(result.Extracted.LegalName))
	fmt.Fprintf(&b, "| DBA name | %s |\n", escape(result.Extracted.DBAName))
	fmt.Fprintf(&b, "| USDOT number | %s |\n", escape(result.Extracted.USDOTNumber))
	fmt.Fprintf(&b, "| Address | %s |\n", escape(result.Extracted.RawAddress))
	fmt.Fprintf(&b, "| Phone | %s |\n", escape(result.Extracted.Contact.Phone))
	fmt.Fprintf(&b, "| Email | %s |\n", escape(result.Extracted.Contact.Email))
	for _, id := range result.Extracted.Identifiers {
		fmt.Fprintf(&b, "| Identifier %s | %s |\n", escape(id.Type), escape(id.Value))
	}
	fmt.Fprintf(&b, "\n## Disclaimer\n\n%s\n", domain.Disclaimer)
	return b.String()
}

func ExtractTable(result ExtractResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "OpenHaul Guard packet extract\n\n")
	fmt.Fprintf(&b, "Packet: %s\n", result.PacketPath)
	fmt.Fprintf(&b, "Legal name: %s\n", blank(result.Extracted.LegalName))
	fmt.Fprintf(&b, "DBA name: %s\n", blank(result.Extracted.DBAName))
	fmt.Fprintf(&b, "USDOT: %s\n", blank(result.Extracted.USDOTNumber))
	for _, id := range result.Extracted.Identifiers {
		fmt.Fprintf(&b, "%s: %s\n", strings.ToUpper(id.Type), blank(id.Value))
	}
	fmt.Fprintf(&b, "Address: %s\n", blank(result.Extracted.RawAddress))
	fmt.Fprintf(&b, "Phone: %s\n", blank(result.Extracted.Contact.Phone))
	fmt.Fprintf(&b, "Email: %s\n", blank(result.Extracted.Contact.Email))
	return b.String()
}

func Markdown(result CheckResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# OpenHaul Guard Packet Check\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", result.GeneratedAt)
	fmt.Fprintf(&b, "Packet: %s\n", result.PacketPath)
	fmt.Fprintf(&b, "Lookup input: %s %s\n", result.Lookup.InputType, result.Lookup.InputValue)
	fmt.Fprintf(&b, "Resolved USDOT: %s\n\n", result.Lookup.ResolvedUSDOT)
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "- Recommendation: %s\n", result.Summary.Recommendation)
	fmt.Fprintf(&b, "- Matches: %d\n", result.Summary.Matches)
	fmt.Fprintf(&b, "- Mismatches: %d\n", result.Summary.Mismatches)
	fmt.Fprintf(&b, "- Missing packet fields: %d\n", result.Summary.MissingPacket)
	fmt.Fprintf(&b, "- Missing source fields: %d\n\n", result.Summary.MissingSource)
	fmt.Fprintf(&b, "## Comparisons\n\n")
	fmt.Fprintf(&b, "| Field | Status | Method | Packet | Source |\n|---|---|---|---|---|\n")
	for _, c := range result.Comparisons {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", escape(c.Field), escape(c.Status), escape(c.Method), escape(c.PacketValue), escape(c.SourceValue))
	}
	fmt.Fprintf(&b, "\n## Disclaimer\n\n%s\n", domain.Disclaimer)
	return b.String()
}

func Table(result CheckResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "OpenHaul Guard packet check\n\n")
	fmt.Fprintf(&b, "Packet: %s\n", result.PacketPath)
	fmt.Fprintf(&b, "Carrier: %s\n", blank(result.Carrier.LegalName))
	fmt.Fprintf(&b, "USDOT: %s\n", blank(result.Carrier.USDOTNumber))
	fmt.Fprintf(&b, "Recommendation: %s\n", result.Summary.Recommendation)
	fmt.Fprintf(&b, "Matches: %d  Mismatches: %d  Missing packet: %d  Missing source: %d\n\n", result.Summary.Matches, result.Summary.Mismatches, result.Summary.MissingPacket, result.Summary.MissingSource)
	for _, c := range result.Comparisons {
		if c.Status == "match" {
			continue
		}
		fmt.Fprintf(&b, "%s: %s", c.Field, c.Status)
		if c.PacketValue != "" || c.SourceValue != "" {
			fmt.Fprintf(&b, " (packet=%q source=%q)", c.PacketValue, c.SourceValue)
		}
		fmt.Fprintln(&b)
	}
	if result.Summary.Mismatches == 0 && result.Summary.MissingPacket == 0 && result.Summary.MissingSource == 0 {
		fmt.Fprintf(&b, "No packet/source differences found.\n")
	}
	return b.String()
}

func firstSubmatch(re *regexp.Regexp, text string) string {
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func cleanValue(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, " \t:-")
	return strings.Join(strings.Fields(s), " ")
}

func extractAddress(text string) string {
	matches := addressRE.FindStringSubmatch(text)
	if len(matches) >= 2 && strings.TrimSpace(matches[1]) != "" {
		return cleanValue(matches[1])
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if lower != "address:" && lower != "physical address:" && lower != "business address:" {
			continue
		}
		var parts []string
		for _, next := range lines[i+1:] {
			next = cleanValue(next)
			if next == "" {
				if len(parts) > 0 {
					break
				}
				continue
			}
			if looksLikeLabel(next) {
				break
			}
			parts = append(parts, next)
			if len(parts) == 2 {
				break
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}
	return ""
}

func looksLikeLabel(s string) bool {
	if !strings.Contains(s, ":") {
		return false
	}
	label := strings.SplitN(s, ":", 2)[0]
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 30 {
		return false
	}
	for _, r := range label {
		if !unicode.IsLetter(r) && !unicode.IsSpace(r) && r != '/' {
			return false
		}
	}
	return true
}

func parseAddress(raw string) domain.Address {
	raw = cleanValue(raw)
	if raw == "" {
		return domain.Address{}
	}
	matches := cityStateRE.FindStringSubmatch(raw)
	if len(matches) != 5 {
		return domain.Address{Line1: raw}
	}
	return domain.Address{
		Line1:      cleanValue(matches[1]),
		City:       cleanValue(matches[2]),
		State:      strings.ToUpper(cleanValue(matches[3])),
		PostalCode: cleanValue(matches[4]),
	}
}

func appendIdentifier(ids []domain.Identifier, typ, value string) []domain.Identifier {
	if typ == "" || value == "" {
		return ids
	}
	key := typ + ":" + value
	for _, id := range ids {
		if strings.ToUpper(id.Type)+":"+digitsOnly(id.Value) == key {
			return ids
		}
	}
	return append(ids, domain.Identifier{Type: typ, Value: value})
}

func compareText(field, packetValue, sourceValue string, allowFuzzy bool) FieldComparison {
	packetValue = cleanValue(packetValue)
	sourceValue = cleanValue(sourceValue)
	base := FieldComparison{Field: field, PacketValue: packetValue, SourceValue: sourceValue}
	if missingStatus(&base) {
		return base
	}
	if packetValue == sourceValue {
		base.Status = "match"
		base.Method = "exact"
		base.Score = 1
		return base
	}
	packetNorm := comparableText(packetValue)
	sourceNorm := comparableText(sourceValue)
	if packetNorm == sourceNorm {
		base.Status = "match"
		base.Method = "normalized"
		base.Score = 1
		return base
	}
	if allowFuzzy {
		score := fuzzyScore(packetNorm, sourceNorm)
		if score >= 0.9 {
			base.Status = "match"
			base.Method = "fuzzy"
			base.Score = score
			return base
		}
		base.Score = score
	}
	base.Status = "mismatch"
	base.Method = "normalized"
	return base
}

func compareAddress(field, packetValue, sourceValue string) FieldComparison {
	packetValue = cleanValue(packetValue)
	sourceValue = cleanValue(sourceValue)
	base := FieldComparison{Field: field, PacketValue: packetValue, SourceValue: sourceValue}
	if missingStatus(&base) {
		return base
	}
	if packetValue == sourceValue {
		base.Status = "match"
		base.Method = "exact"
		base.Score = 1
		return base
	}
	packetNorm := comparableAddress(packetValue)
	sourceNorm := comparableAddress(sourceValue)
	if packetNorm == sourceNorm {
		base.Status = "match"
		base.Method = "normalized"
		base.Score = 1
		return base
	}
	score := fuzzyScore(packetNorm, sourceNorm)
	if score >= 0.92 {
		base.Status = "match"
		base.Method = "fuzzy"
		base.Score = score
		return base
	}
	base.Status = "mismatch"
	base.Method = "normalized"
	base.Score = score
	return base
}

func compareDigits(field, packetValue, sourceValue string) FieldComparison {
	packetDigits := digitsOnly(packetValue)
	sourceDigits := digitsOnly(sourceValue)
	base := FieldComparison{Field: field, PacketValue: packetDigits, SourceValue: sourceDigits}
	if missingStatus(&base) {
		return base
	}
	if packetDigits == sourceDigits {
		base.Status = "match"
		base.Method = "exact"
		base.Score = 1
		return base
	}
	base.Status = "mismatch"
	base.Method = "exact"
	return base
}

func comparePhone(field, packetValue, sourceValue string) FieldComparison {
	packetPhone := normalize.Phone(packetValue)
	sourcePhone := normalize.Phone(sourceValue)
	base := FieldComparison{Field: field, PacketValue: packetPhone, SourceValue: sourcePhone}
	if missingStatus(&base) {
		return base
	}
	if digitsOnly(packetPhone) == digitsOnly(sourcePhone) {
		base.Status = "match"
		base.Method = "normalized"
		base.Score = 1
		return base
	}
	base.Status = "mismatch"
	base.Method = "normalized"
	return base
}

func compareEmail(field, packetValue, sourceValue string) FieldComparison {
	packetEmail := strings.ToLower(strings.TrimSpace(packetValue))
	sourceEmail := strings.ToLower(strings.TrimSpace(sourceValue))
	base := FieldComparison{Field: field, PacketValue: packetEmail, SourceValue: sourceEmail}
	if missingStatus(&base) {
		return base
	}
	if packetEmail == sourceEmail {
		base.Status = "match"
		base.Method = "normalized"
		base.Score = 1
		return base
	}
	base.Status = "mismatch"
	base.Method = "normalized"
	return base
}

func missingStatus(c *FieldComparison) bool {
	switch {
	case c.PacketValue == "" && c.SourceValue == "":
		c.Status = "missing_both"
		c.Method = "absent"
		return true
	case c.PacketValue == "":
		c.Status = "missing_packet"
		c.Method = "absent"
		return true
	case c.SourceValue == "":
		c.Status = "missing_source"
		c.Method = "absent"
		return true
	default:
		return false
	}
}

func summarize(comparisons []FieldComparison) Summary {
	var summary Summary
	for _, comparison := range comparisons {
		switch comparison.Status {
		case "match":
			summary.Matches++
		case "mismatch":
			summary.Mismatches++
		case "missing_packet":
			summary.MissingPacket++
		case "missing_source":
			summary.MissingSource++
		}
	}
	switch {
	case summary.Mismatches > 0:
		summary.Recommendation = "manual_review_recommended"
	case summary.MissingPacket > 0 || summary.MissingSource > 0:
		summary.Recommendation = "review_missing_fields"
	default:
		summary.Recommendation = "packet_matches_lookup"
	}
	return summary
}

func identifiersByType(ids []domain.Identifier) map[string]string {
	out := map[string]string{}
	for _, id := range ids {
		typ := strings.ToUpper(strings.TrimSpace(id.Type))
		value := digitsOnly(id.Value)
		if typ == "" || value == "" || out[typ] != "" {
			continue
		}
		out[typ] = value
	}
	return out
}

func comparableText(s string) string {
	return normalize.ComparableString(cleanValue(s))
}

func comparableAddress(s string) string {
	s = comparableText(s)
	replacer := strings.NewReplacer(
		" street", " st",
		" avenue", " ave",
		" road", " rd",
		" drive", " dr",
		" boulevard", " blvd",
		" lane", " ln",
		" court", " ct",
		" suite", " ste",
		" tennessee", " tn",
	)
	s = replacer.Replace(" " + s)
	return strings.TrimSpace(s)
}

func fuzzyScore(left, right string) float64 {
	leftTokens := tokenSet(left)
	rightTokens := tokenSet(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range leftTokens {
		if rightTokens[token] {
			intersection++
		}
	}
	union := len(leftTokens) + len(rightTokens) - intersection
	jaccard := float64(intersection) / float64(union)
	containment := float64(intersection) / float64(min(len(leftTokens), len(rightTokens)))
	if containment == 1 && min(len(leftTokens), len(rightTokens)) >= 2 {
		return max(jaccard, 0.9)
	}
	return jaccard
}

func tokenSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.Fields(s) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		out[token] = true
	}
	return out
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

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
