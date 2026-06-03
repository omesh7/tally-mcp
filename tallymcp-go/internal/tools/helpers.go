// Package tools implements MCP tool handlers that bridge AI clients to TallyPrime.
//
// Each file in this package contains handlers for a logical domain:
//   - companies.go  → company listing
//   - ledgers.go    → trial balance, ledger balances
//   - stock.go      → stock/inventory items
//   - vouchers.go   → voucher creation (write operations)
//   - analytics.go  → sales and expense analytics
//   - register.go   → wires all tools to the MCP server
package tools

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/omesh7/tally-mcp/internal/tally"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

// tallyClient is the shared Tally HTTP client used by all tool handlers.
// Set during RegisterAll().
var tallyClient *tally.Client

// ──────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ──────────────────────────────────────────────────────────────────────────────

// postAndParse sends an XML query to Tally and returns the parsed element tree.
func postAndParse(ctx context.Context, xml string) (*tally.GenericElement, error) {
	resp, err := tallyClient.PostXML(ctx, xml)
	if err != nil {
		return nil, err
	}
	return tally.ParseXML(resp)
}

// formatCurrency formats a float as an Indian Rupee string with comma separators.
// Negative values are prefixed with a minus sign.
func formatCurrency(amount float64) string {
	negative := amount < 0
	abs := math.Abs(amount)
	// Simple comma formatting
	s := fmt.Sprintf("%.2f", abs)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := parts[1]

	// Indian numbering: last 3 digits, then groups of 2
	if len(intPart) <= 3 {
		if negative {
			return "-" + intPart + "." + decPart
		}
		return intPart + "." + decPart
	}

	result := intPart[len(intPart)-3:]
	remaining := intPart[:len(intPart)-3]
	for len(remaining) > 2 {
		result = remaining[len(remaining)-2:] + "," + result
		remaining = remaining[:len(remaining)-2]
	}
	if len(remaining) > 0 {
		result = remaining + "," + result
	}

	if negative {
		return "-" + result + "." + decPart
	}
	return result + "." + decPart
}

// parseFloat safely parses a string to float64, returning 0 on failure.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func sameName(a, b string) bool {
	return normalizeName(a) == normalizeName(b)
}

func nonEmptyTrimmed(values []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := normalizeName(v)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

func ledgerFilterFormula(ledgers []string) string {
	ledgers = nonEmptyTrimmed(ledgers)
	if len(ledgers) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ledgers))
	for _, ledger := range ledgers {
		parts = append(parts, tallyxml.EqualFilter("LedgerName", ledger))
	}
	return strings.Join(parts, " or ")
}

func cleanDateYYYYMMDD(date string) (string, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return "", nil
	}
	if len(date) == 8 && strings.IndexFunc(date, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		if _, err := time.Parse("20060102", date); err != nil {
			return "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD or YYYYMMDD", date)
		}
		return date, nil
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD or YYYYMMDD", date)
	}
	return strings.ReplaceAll(date, "-", ""), nil
}

func dateRangeStaticVars(startDate, endDate string) (map[string]string, error) {
	staticVars := map[string]string{
		"SVCurrentCompany": "##SVCurrentCompany",
		"SVTargetCompany":  "##SVCurrentCompany",
	}
	from, err := cleanDateYYYYMMDD(startDate)
	if err != nil {
		return nil, err
	}
	to, err := cleanDateYYYYMMDD(endDate)
	if err != nil {
		return nil, err
	}
	if from != "" && to == "" {
		to = from
	}
	if to != "" && from == "" {
		from = to
	}
	if from != "" && to != "" {
		if from > to {
			return nil, fmt.Errorf("start_date must be on or before end_date")
		}
		staticVars["SVFROMDATE"] = from
		staticVars["SVTODATE"] = to
	}
	return staticVars, nil
}

// buildStandardExportQuery builds a common TDL export query XML string.
// This is the pattern used by most read tools.
func buildStandardExportQuery(tdl *tallyxml.TDL, extraStaticVars map[string]string) string {
	env := tallyxml.NewExportEnvelope()
	for k, v := range extraStaticVars {
		env.WithStaticVar(k, v)
	}
	return env.WithTDL(tdl).Build()
}

func resolveLedgerName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return name, nil
	}
	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		return name, err
	}
	for _, l := range ledgers {
		if strings.EqualFold(l.Name, name) {
			return l.Name, nil
		}
	}
	return name, nil
}

func resolveStockItemName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return name, nil
	}
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN").
		Field("FN", "Name", tallyxml.FormulaName).
		Collection("Col", tallyxml.CollectionStockItem)

	xml := buildStandardExportQuery(tdl, nil)
	root, err := postAndParse(ctx, xml)
	if err != nil {
		return name, err
	}

	for _, row := range root.FindAll("ROW") {
		item := row.FindText("Name")
		if strings.EqualFold(item, name) {
			return item, nil
		}
	}
	return name, nil
}

func parseTallyDate(dateStr string) (time.Time, bool) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}, false
	}
	clean := strings.ReplaceAll(strings.ReplaceAll(dateStr, "-", ""), "/", "")
	if len(clean) == 8 && strings.IndexFunc(clean, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		if t, err := time.Parse("20060102", clean); err == nil {
			return t, true
		}
	}
	for _, layout := range []string{
		"2006-01-02",
		"2006/01/02",
		"2-Jan-06",
		"02-Jan-06",
		"2-Jan-2006",
		"02-Jan-2006",
		"2/1/2006",
		"02/01/2006",
	} {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func isInDateRange(tallyDate, from, to string) bool {
	if tallyDate == "" {
		return false
	}
	t, ok := parseTallyDate(tallyDate)
	if !ok {
		return true // Fallback to including if we can't parse it
	}
	clean := t.Format("20060102")
	if from != "" && clean < from {
		return false
	}
	if to != "" && clean > to {
		return false
	}
	return true
}

// Ensure imports are used
var _ = tallyxml.CollectionLedger
