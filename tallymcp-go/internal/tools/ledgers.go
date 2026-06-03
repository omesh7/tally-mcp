package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tallymcp/internal/xml"
)

// ──────────────────────────────────────────────────────────────────────────────
// get_trial_balance
// ──────────────────────────────────────────────────────────────────────────────

// TrialBalanceInput is the input schema for get_trial_balance (no parameters).
type TrialBalanceInput struct{}

// GetTrialBalance fetches a summarized trial balance showing debit and credit balances
// for all active ledger accounts in the company.
func GetTrialBalance(ctx context.Context, req *mcp.CallToolRequest, input TrialBalanceInput) (*mcp.CallToolResult, any, error) {
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FP", "FB", "FD").
		Field("FN", "Name", tallyxml.FormulaName).
		Field("FP", "Parent", tallyxml.FormulaParent).
		Field("FB", "Balance", tallyxml.FormulaClosingBalance).
		Field("FD", "Type", tallyxml.FormulaIsDebit).
		Collection("Col", tallyxml.CollectionLedger)

	xml := buildStandardExportQuery(tdl, nil)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error reading Trial Balance: %v", err)), nil, nil
	}

	type entry struct {
		name, parent string
		balance      float64
	}

	var debits, credits []entry
	var totalDr, totalCr float64

	for _, row := range root.FindAll("ROW") {
		name := row.FindText("Name")
		parent := row.FindText("Parent")
		bal := math.Abs(parseFloat(row.FindText("Balance")))
		btype := row.FindText("Type")

		if bal == 0 {
			continue
		}

		if btype == "Dr" {
			debits = append(debits, entry{name, parent, bal})
			totalDr += bal
		} else {
			credits = append(credits, entry{name, parent, bal})
			totalCr += bal
		}
	}

	var sb strings.Builder
	sb.WriteString("### Trial Balance Summary\n\n")
	sb.WriteString("| Ledger Name | Group / Parent | Debit Balance (Rs) | Credit Balance (Rs) |\n")
	sb.WriteString("| :--- | :--- | :---: | :---: |\n")

	for _, e := range debits {
		sb.WriteString(fmt.Sprintf("| %s | *%s* | %s | |\n", e.name, e.parent, formatCurrency(e.balance)))
	}
	for _, e := range credits {
		sb.WriteString(fmt.Sprintf("| %s | *%s* | | %s |\n", e.name, e.parent, formatCurrency(e.balance)))
	}
	sb.WriteString(fmt.Sprintf("| **TOTAL** | | **%s** | **%s** |\n", formatCurrency(totalDr), formatCurrency(totalCr)))

	return textResult(sb.String()), nil, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// get_ledger_closing_balance
// ──────────────────────────────────────────────────────────────────────────────

// LedgerBalanceInput is the input schema for get_ledger_closing_balance.
type LedgerBalanceInput struct {
	LedgerName string `json:"ledger_name" jsonschema:"The exact name of the ledger (e.g. HDFC Current Account)"`
	Date       string `json:"date,omitempty" jsonschema:"Optional date in YYYY-MM-DD format to retrieve balance as of that date"`
}

// GetLedgerClosingBalance retrieves the closing balance of a specific ledger account.
func GetLedgerClosingBalance(ctx context.Context, req *mcp.CallToolRequest, input LedgerBalanceInput) (*mcp.CallToolResult, any, error) {
	if input.LedgerName == "" {
		return textResult("❌ **Error:** ledger_name is a required parameter."), nil, nil
	}

	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "F_BAL", "F_DEBIT").
		Field("FN", "Name", tallyxml.FormulaName).
		Field("F_BAL", "Balance", tallyxml.FormulaClosingBalance).
		Field("F_DEBIT", "Type", tallyxml.FormulaIsDebitFull).
		CollectionWithFilter("Col", tallyxml.CollectionLedger, "NameFilter").
		Filter("NameFilter", tallyxml.EqualFilter("Name", input.LedgerName))

	// Build static vars with optional date filter
	staticVars := map[string]string{
		"SVCurrentCompany": "##SVCurrentCompany",
		"SVTargetCompany":  "##SVCurrentCompany",
	}
	if input.Date != "" {
		clean := strings.ReplaceAll(input.Date, "-", "")
		if len(clean) == 8 {
			staticVars["SVTOYEAR"] = clean[:4]
			staticVars["SVTOMONTH"] = clean[4:6]
			staticVars["SVTODAY"] = clean[6:8]
		}
	}

	xml := buildStandardExportQuery(tdl, staticVars)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error retrieving ledger closing balance: %v", err)), nil, nil
	}

	rows := root.FindAll("ROW")
	if len(rows) == 0 {
		return textResult(fmt.Sprintf("Ledger '%s' was not found in Tally Prime. Please verify the exact name.", input.LedgerName)), nil, nil
	}

	row := rows[0]
	name := row.FindText("Name")
	bal := math.Abs(parseFloat(row.FindText("Balance")))
	btype := row.FindText("Type")

	dateInfo := ""
	if input.Date != "" {
		dateInfo = fmt.Sprintf(" as on %s", input.Date)
	}

	return textResult(fmt.Sprintf("**Closing Balance for '%s'%s:**\nRs %s (%s)", name, dateInfo, formatCurrency(bal), btype)), nil, nil
}

// ListLedgersInput is the input schema for list_ledgers.
type ListLedgersInput struct {
	Group string `json:"group,omitempty" jsonschema:"Optional parent group to filter ledgers (e.g. 'Sundry Debtors', 'Bank Accounts')"`
}

// ListLedgers lists all ledgers in the active company, with an optional group filter.
func ListLedgers(ctx context.Context, req *mcp.CallToolRequest, input ListLedgersInput) (*mcp.CallToolResult, any, error) {
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FP", "FB", "FD").
		Field("FN", "Name", tallyxml.FormulaName).
		Field("FP", "Parent", tallyxml.FormulaParent).
		Field("FB", "Balance", tallyxml.FormulaClosingBalance).
		Field("FD", "Type", tallyxml.FormulaIsDebit).
		Collection("Col", tallyxml.CollectionLedger)

	xml := buildStandardExportQuery(tdl, nil)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error listing ledgers: %v", err)), nil, nil
	}

	var sb strings.Builder
	sb.WriteString("### Ledgers in Tally Prime:\n\n")
	sb.WriteString("| Ledger Name | Group / Parent | Closing Balance (Rs) |\n")
	sb.WriteString("| :--- | :--- | :---: |\n")

	count := 0
	for _, row := range root.FindAll("ROW") {
		name := row.FindText("Name")
		parent := row.FindText("Parent")
		bal := math.Abs(parseFloat(row.FindText("Balance")))
		btype := row.FindText("Type")

		// Filter by group if specified
		if input.Group != "" && !strings.EqualFold(parent, input.Group) {
			continue
		}

		balStr := "-"
		if bal > 0 {
			balStr = fmt.Sprintf("%s (%s)", formatCurrency(bal), btype)
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, parent, balStr))
		count++
	}

	if count == 0 {
		return textResult("No ledgers found matching the criteria."), nil, nil
	}

	return textResult(sb.String()), nil, nil
}
