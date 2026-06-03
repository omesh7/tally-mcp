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

	"github.com/omesh7/tallymcp/internal/tally"
	tallyxml "github.com/omesh7/tallymcp/internal/xml"
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

// formatCurrency formats a float as Indian Rupee string with commas.
func formatCurrency(amount float64) string {
	abs := math.Abs(amount)
	// Simple comma formatting
	s := fmt.Sprintf("%.2f", abs)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := parts[1]

	// Indian numbering: last 3 digits, then groups of 2
	if len(intPart) <= 3 {
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

// buildStandardExportQuery builds a common TDL export query XML string.
// This is the pattern used by most read tools.
func buildStandardExportQuery(tdl *tallyxml.TDL, extraStaticVars map[string]string) string {
	env := tallyxml.NewExportEnvelope()
	for k, v := range extraStaticVars {
		env.WithStaticVar(k, v)
	}
	return env.WithTDL(tdl).Build()
}

// Ensure imports are used
var _ = tallyxml.CollectionLedger
