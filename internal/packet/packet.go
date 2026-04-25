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
	LegalName         string              `json:"legal_name,omitempty"`
	DBAName           string              `json:"dba_name,omitempty"`
	USDOTNumber       string              `json:"usdot_number,omitempty"`
	Identifiers       []domain.Identifier `json:"identifiers,omitempty"`
	Address           domain.Address      `json:"address,omitempty"`
	RawAddress        string              `json:"raw_address,omitempty"`
	Contact           domain.Contact      `json:"contact,omitempty"`
	Insurance         *ExtractedInsurance `json:"insurance,omitempty"`
	CertificateHolder string              `json:"certificate_holder,omitempty"`
	RemitTo           string              `json:"remit_to,omitempty"`
	Payee             string              `json:"payee,omitempty"`
	FactoringCompany  string              `json:"factoring_company,omitempty"`
}

type ExtractedInsurance struct {
	Carrier        string `json:"carrier,omitempty"`
	PolicyNumber   string `json:"policy_number,omitempty"`
	EffectiveDate  string `json:"effective_date,omitempty"`
	ExpirationDate string `json:"expiration_date,omitempty"`
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

	insuranceCarrierRE = regexp.MustCompile(`(?im)^\s*(?:insurance\s+(?:carrier|company|provider)|liability\s+(?:carrier|insurer)|cargo\s+(?:carrier|insurer)|insurer|underwriter)\s*[:\-]\s*(.+?)\s*$`)
	insurerLetterRE    = regexp.MustCompile(`(?im)^\s*insurer\s+[A-Z]\s*:?\s*(.+?)\s*$`)
	policyNumberRE     = regexp.MustCompile(`(?im)^\s*(?:insurance\s+)?policy\s*(?:number|no\.?|#)?\s*[:#\-]\s*(.+?)\s*$`)
	policyPeriodRE     = regexp.MustCompile(`(?im)^\s*(?:policy|coverage)\s+(?:term|period)\s*[:\-]\s*(.+?)\s*$`)
	dateTokenRE        = regexp.MustCompile(`(?i)\b(?:\d{4}-\d{1,2}-\d{1,2}|\d{1,2}[/-]\d{1,2}[/-]\d{2,4}|[A-Z]{3,9}\.?\s+\d{1,2},?\s+\d{4}|\d{1,2}\s+[A-Z]{3,9}\.?\s+\d{4})\b`)
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
	out.Insurance = extractInsurance(text)
	out.CertificateHolder = extractCertificateHolder(text)
	out.RemitTo = extractBlock(text, []string{"remit to", "remit-to", "remittance address", "payment address"}, nil, 4)
	out.Payee = extractBlock(text, []string{"payee", "pay to", "checks payable to", "make payable to"}, nil, 2)
	out.FactoringCompany = extractBlock(text, []string{"factoring company", "factor", "factoring"}, nil, 3)
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
	out = append(out, compareInsurance(packetFields.Insurance, carrier.Insurance)...)

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
	insurance := insuranceFields(result.Extracted.Insurance)
	fmt.Fprintf(&b, "| Insurance carrier | %s |\n", escape(insurance.Carrier))
	fmt.Fprintf(&b, "| Insurance policy number | %s |\n", escape(insurance.PolicyNumber))
	fmt.Fprintf(&b, "| Insurance effective date | %s |\n", escape(insurance.EffectiveDate))
	fmt.Fprintf(&b, "| Insurance expiration date | %s |\n", escape(insurance.ExpirationDate))
	fmt.Fprintf(&b, "| Certificate holder | %s |\n", escape(result.Extracted.CertificateHolder))
	fmt.Fprintf(&b, "| Remit to | %s |\n", escape(result.Extracted.RemitTo))
	fmt.Fprintf(&b, "| Payee | %s |\n", escape(result.Extracted.Payee))
	fmt.Fprintf(&b, "| Factoring company | %s |\n", escape(result.Extracted.FactoringCompany))
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
	insurance := insuranceFields(result.Extracted.Insurance)
	fmt.Fprintf(&b, "Insurance carrier: %s\n", blank(insurance.Carrier))
	fmt.Fprintf(&b, "Insurance policy number: %s\n", blank(insurance.PolicyNumber))
	fmt.Fprintf(&b, "Insurance effective date: %s\n", blank(insurance.EffectiveDate))
	fmt.Fprintf(&b, "Insurance expiration date: %s\n", blank(insurance.ExpirationDate))
	fmt.Fprintf(&b, "Certificate holder: %s\n", blank(result.Extracted.CertificateHolder))
	fmt.Fprintf(&b, "Remit to: %s\n", blank(result.Extracted.RemitTo))
	fmt.Fprintf(&b, "Payee: %s\n", blank(result.Extracted.Payee))
	fmt.Fprintf(&b, "Factoring company: %s\n", blank(result.Extracted.FactoringCompany))
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

func extractInsurance(text string) *ExtractedInsurance {
	out := ExtractedInsurance{
		Carrier:       cleanValue(firstSubmatch(insuranceCarrierRE, text)),
		PolicyNumber:  cleanPolicyNumber(firstSubmatch(policyNumberRE, text)),
		EffectiveDate: extractDateAfterLabel(text, []string{"effective date", "policy effective", "policy effective date", "eff date", "eff. date"}),
		ExpirationDate: extractDateAfterLabel(text, []string{
			"expiration date",
			"expiry date",
			"policy expiration",
			"policy expiration date",
			"policy exp",
			"exp date",
			"exp. date",
			"expires",
		}),
	}
	if out.Carrier == "" {
		out.Carrier = cleanValue(firstSubmatch(insurerLetterRE, text))
	}
	if period := firstSubmatch(policyPeriodRE, text); period != "" {
		dates := dateTokens(period)
		if out.EffectiveDate == "" && len(dates) > 0 {
			out.EffectiveDate = normalizeDate(dates[0])
		}
		if out.ExpirationDate == "" && len(dates) > 1 {
			out.ExpirationDate = normalizeDate(dates[1])
		}
	}
	fillInsuranceFromPolicyTable(text, &out)
	if out.Carrier == "" && out.PolicyNumber == "" && out.EffectiveDate == "" && out.ExpirationDate == "" {
		return nil
	}
	return &out
}

func fillInsuranceFromPolicyTable(text string, out *ExtractedInsurance) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		heading := strings.ToLower(line)
		if !(strings.Contains(heading, "policy") && (strings.Contains(heading, "eff") || strings.Contains(heading, "exp"))) &&
			!strings.Contains(heading, "type of insurance") {
			continue
		}
		for _, next := range lines[i+1 : min(len(lines), i+7)] {
			dates := dateTokens(next)
			if len(dates) < 2 {
				continue
			}
			if out.EffectiveDate == "" {
				out.EffectiveDate = normalizeDate(dates[0])
			}
			if out.ExpirationDate == "" {
				out.ExpirationDate = normalizeDate(dates[1])
			}
			if out.PolicyNumber == "" {
				out.PolicyNumber = policyNumberBeforeDate(next, dates[0])
			}
			return
		}
	}
}

func policyNumberBeforeDate(line, firstDate string) string {
	before, _, _ := strings.Cut(line, firstDate)
	fields := strings.Fields(before)
	for i := len(fields) - 1; i >= 0; i-- {
		candidate := cleanPolicyNumber(fields[i])
		if len(normalizePolicyNumber(candidate)) >= 4 {
			return candidate
		}
	}
	return ""
}

func extractCertificateHolder(text string) string {
	return extractBlock(text, []string{"certificate holder", "cert holder"}, []string{
		"cancellation",
		"authorized representative",
		"description of operations",
		"insurer",
		"coverage",
	}, 5)
}

func extractDateAfterLabel(text string, labels []string) string {
	for _, line := range strings.Split(text, "\n") {
		if value, ok := valueAfterLabel(line, labels); ok {
			if date := firstDate(value); date != "" {
				return normalizeDate(date)
			}
		}
	}
	return ""
}

func extractBlock(text string, labels []string, stopLabels []string, maxLines int) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if value, ok := valueAfterLabel(line, labels); ok {
			if cleaned := cleanValue(value); cleaned != "" {
				return cleaned
			}
			return collectBlock(lines[i+1:], stopLabels, maxLines)
		}
		if lineIsLabel(line, labels) {
			return collectBlock(lines[i+1:], stopLabels, maxLines)
		}
	}
	return ""
}

func collectBlock(lines []string, stopLabels []string, maxLines int) string {
	var parts []string
	for _, line := range lines {
		cleaned := cleanValue(line)
		if cleaned == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if lineIsLabel(cleaned, stopLabels) || looksLikeLabel(cleaned) {
			break
		}
		parts = append(parts, cleaned)
		if len(parts) == maxLines {
			break
		}
	}
	return strings.Join(parts, ", ")
}

func valueAfterLabel(line string, labels []string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	for _, label := range labels {
		pattern := `(?i)^\s*` + flexibleLabelPattern(label) + `\s*(?::|\-|\x{2013}|\x{2014})\s*(.*?)\s*$`
		matches := regexp.MustCompile(pattern).FindStringSubmatch(trimmed)
		if len(matches) == 2 {
			return matches[1], true
		}
	}
	return "", false
}

func lineIsLabel(line string, labels []string) bool {
	normalizedLine := normalizeLabel(line)
	for _, label := range labels {
		if normalizedLine == normalizeLabel(label) {
			return true
		}
	}
	return false
}

func flexibleLabelPattern(label string) string {
	parts := strings.FieldsFunc(strings.ToLower(label), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var escaped []string
	for _, part := range parts {
		if part != "" {
			escaped = append(escaped, regexp.QuoteMeta(part))
		}
	}
	return strings.Join(escaped, `[\s\-/]*`)
}

func normalizeLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, " \t:-#")
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.Join(fields, " ")
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

func compareInsurance(packetInsurance *ExtractedInsurance, sourceInsurance []domain.InsuranceRecord) []FieldComparison {
	packet := insuranceFields(packetInsurance)
	source, ok := selectInsuranceRecord(packetInsurance, sourceInsurance)
	if !ok && packet.Carrier == "" && packet.PolicyNumber == "" && packet.EffectiveDate == "" && packet.ExpirationDate == "" {
		return nil
	}

	var out []FieldComparison
	out = appendRelevant(out, compareText("insurance.carrier", packet.Carrier, source.InsurerName, true))
	out = appendRelevant(out, comparePolicyNumber("insurance.policy_number", packet.PolicyNumber, source.PolicyNumber))
	out = appendRelevant(out, compareDate("insurance.effective_date", packet.EffectiveDate, source.EffectiveDate))
	out = appendRelevant(out, compareDate("insurance.expiration_date", packet.ExpirationDate, sourceInsuranceExpiration(source)))
	return out
}

func appendRelevant(comparisons []FieldComparison, comparison FieldComparison) []FieldComparison {
	if comparison.Status == "missing_both" {
		return comparisons
	}
	return append(comparisons, comparison)
}

func selectInsuranceRecord(packetInsurance *ExtractedInsurance, records []domain.InsuranceRecord) (domain.InsuranceRecord, bool) {
	if len(records) == 0 {
		return domain.InsuranceRecord{}, false
	}
	packet := insuranceFields(packetInsurance)
	bestScore := -1
	bestIndex := -1
	for i, record := range records {
		if !insuranceRecordHasData(record) {
			continue
		}
		score := 0
		if packet.PolicyNumber != "" && record.PolicyNumber != "" && normalizePolicyNumber(packet.PolicyNumber) == normalizePolicyNumber(record.PolicyNumber) {
			score += 100
		}
		if packet.Carrier != "" && record.InsurerName != "" {
			packetCarrier := comparableText(packet.Carrier)
			sourceCarrier := comparableText(record.InsurerName)
			switch {
			case packetCarrier == sourceCarrier:
				score += 30
			case fuzzyScore(packetCarrier, sourceCarrier) >= 0.9:
				score += 20
			}
		}
		if packet.EffectiveDate != "" && record.EffectiveDate != "" && normalizeDate(packet.EffectiveDate) == normalizeDate(record.EffectiveDate) {
			score += 10
		}
		if packet.ExpirationDate != "" && sourceInsuranceExpiration(record) != "" && normalizeDate(packet.ExpirationDate) == normalizeDate(sourceInsuranceExpiration(record)) {
			score += 10
		}
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex >= 0 {
		return records[bestIndex], true
	}
	return domain.InsuranceRecord{}, false
}

func insuranceRecordHasData(record domain.InsuranceRecord) bool {
	return strings.TrimSpace(record.InsurerName) != "" ||
		strings.TrimSpace(record.PolicyNumber) != "" ||
		strings.TrimSpace(record.EffectiveDate) != "" ||
		strings.TrimSpace(record.CancellationDate) != "" ||
		strings.TrimSpace(record.CancelEffectiveDate) != ""
}

func sourceInsuranceExpiration(record domain.InsuranceRecord) string {
	if strings.TrimSpace(record.CancelEffectiveDate) != "" {
		return record.CancelEffectiveDate
	}
	return record.CancellationDate
}

func insuranceFields(insurance *ExtractedInsurance) ExtractedInsurance {
	if insurance == nil {
		return ExtractedInsurance{}
	}
	return *insurance
}

func comparePolicyNumber(field, packetValue, sourceValue string) FieldComparison {
	packetValue = cleanPolicyNumber(packetValue)
	sourceValue = cleanPolicyNumber(sourceValue)
	base := FieldComparison{Field: field, PacketValue: packetValue, SourceValue: sourceValue}
	if missingStatus(&base) {
		return base
	}
	if normalizePolicyNumber(packetValue) == normalizePolicyNumber(sourceValue) {
		base.Status = "match"
		base.Method = "normalized"
		base.Score = 1
		return base
	}
	base.Status = "mismatch"
	base.Method = "normalized"
	return base
}

func compareDate(field, packetValue, sourceValue string) FieldComparison {
	packetDate := normalizeDate(packetValue)
	sourceDate := normalizeDate(sourceValue)
	base := FieldComparison{Field: field, PacketValue: packetDate, SourceValue: sourceDate}
	if missingStatus(&base) {
		return base
	}
	if packetDate == sourceDate {
		base.Status = "match"
		base.Method = "normalized"
		base.Score = 1
		return base
	}
	base.Status = "mismatch"
	base.Method = "normalized"
	return base
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

func cleanPolicyNumber(s string) string {
	s = cleanValue(s)
	s = strings.Trim(s, " \t#")
	if date := firstDate(s); date != "" {
		before, _, _ := strings.Cut(s, date)
		if strings.TrimSpace(before) != "" {
			s = before
		}
	}
	return strings.Trim(s, " \t#:-")
}

func normalizePolicyNumber(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstDate(s string) string {
	dates := dateTokens(s)
	if len(dates) == 0 {
		return ""
	}
	return dates[0]
}

func dateTokens(s string) []string {
	return dateTokenRE.FindAllString(s, -1)
}

func normalizeDate(s string) string {
	s = cleanValue(s)
	if s == "" {
		return ""
	}
	if date := firstDate(s); date != "" {
		s = date
	}
	s = strings.Trim(s, " \t,.;")
	layouts := []string{
		"2006-01-02",
		"2006-1-2",
		"01/02/2006",
		"1/2/2006",
		"01-02-2006",
		"1-2-2006",
		"01/02/06",
		"1/2/06",
		"01-02-06",
		"1-2-06",
		"Jan 2, 2006",
		"January 2, 2006",
		"Jan 2 2006",
		"January 2 2006",
		"2 Jan 2006",
		"02 Jan 2006",
		"2 January 2006",
		"02 January 2006",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return s
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
