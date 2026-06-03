package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

// LedgerStatementInput is the input schema for get_ledger_statement.
type LedgerStatementInput struct {
	LedgerName string `json:"ledger_name" jsonschema:"The exact ledger name to retrieve transactions for."`
	StartDate  string `json:"start_date,omitempty" jsonschema:"Optional start date in YYYY-MM-DD format."`
	EndDate    string `json:"end_date,omitempty" jsonschema:"Optional end date in YYYY-MM-DD format."`
}

// GetLedgerStatement lists vouchers affecting a ledger over an optional date range.
func GetLedgerStatement(ctx context.Context, req *mcp.CallToolRequest, input LedgerStatementInput) (*mcp.CallToolResult, any, error) {
	input.LedgerName = strings.TrimSpace(input.LedgerName)
	if input.LedgerName == "" {
		return textResult("**Error:** ledger_name is a required parameter."), nil, nil
	}

	resolved, err := resolveLedgerName(ctx, input.LedgerName)
	if err == nil {
		input.LedgerName = resolved
	}

	staticVars, err := dateRangeStaticVars(input.StartDate, input.EndDate)
	if err != nil {
		return textResult(fmt.Sprintf("Error reading ledger statement: %v", err)), nil, nil
	}

	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FM", "FD", "FN", "FT", "FA", "F_NARR").
		Field("FM", "MasterID", "$MasterID").
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FT", "Type", "$VoucherTypeName").
		Field("FA", "Amount", `$$NumValue:$AllLedgerEntries[1,@@FilterLedger].Amount`).
		Field("F_NARR", "Narration", "$Narration")

	// Use date comparison range filtering.
	if input.StartDate != "" || input.EndDate != "" {
		tdl.CollectionWithFilterAndFetch("Col", tallyxml.CollectionVoucher, "DateFilter", "AllLedgerEntries").
			Filter("DateFilter", `($Date >= ##SVFROMDATE) and ($Date <= ##SVTODATE)`)
	} else {
		tdl.CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries")
	}
	tdl.FilterFormulae("FilterLedger", tallyxml.EqualFilter("LedgerName", input.LedgerName))

	root, err := postAndParse(ctx, buildStandardExportQuery(tdl, staticVars))
	if err != nil {
		return textResult(fmt.Sprintf("Error reading ledger statement: %v", err)), nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Ledger Statement: %s\n\n", input.LedgerName))
	if input.StartDate != "" || input.EndDate != "" {
		sb.WriteString(fmt.Sprintf("*Date range: %s to %s*\n\n", input.StartDate, input.EndDate))
	}
	sb.WriteString("| Date | Voucher No | Type | Debit (Rs) | Credit (Rs) | Narration |\n")
	sb.WriteString("| :--- | :--- | :--- | :---: | :---: | :--- |\n")

	from, _ := cleanDateYYYYMMDD(input.StartDate)
	to, _ := cleanDateYYYYMMDD(input.EndDate)

	count := 0
	seen := make(map[string]bool)
	for _, row := range root.FindAll("ROW") {
		date := row.FindText("Date")
		if !isInDateRange(date, from, to) {
			continue
		}

		amount := parseFloat(row.FindText("Amount"))
		if amount == 0 {
			continue
		}

		vchNo := row.FindText("VchNo")
		vchType := row.FindText("Type")
		narr := strings.TrimSpace(row.FindText("Narration"))
		if vchNo == "" {
			vchNo = "(Auto)"
		}
		if narr == "" || narr == "**" {
			narr = "no narration"
		}

		key := row.FindText("MasterID")
		if key == "" || key == "0" {
			key = strings.Join([]string{date, vchNo, vchType, fmt.Sprintf("%.2f", amount), narr}, "|")
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		debit, credit := "", ""
		if amount < 0 {
			debit = formatCurrency(math.Abs(amount))
		} else {
			credit = formatCurrency(amount)
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | *%s* |\n",
			date, vchNo, vchType, debit, credit, narr))
		count++
	}

	if count == 0 {
		return textResult(fmt.Sprintf("No vouchers found for ledger '%s' in the specified period.", input.LedgerName)), nil, nil
	}

	return textResult(sb.String()), nil, nil
}

// StockMovementInput is the input schema for get_stock_movement.
type StockMovementInput struct {
	StockItemName string `json:"stock_item_name" jsonschema:"The exact stock item name to retrieve movement for."`
	StartDate     string `json:"start_date,omitempty" jsonschema:"Optional start date in YYYY-MM-DD format."`
	EndDate       string `json:"end_date,omitempty" jsonschema:"Optional end date in YYYY-MM-DD format."`
}

// GetStockMovement summarizes inward and outward quantity for a stock item.
func GetStockMovement(ctx context.Context, req *mcp.CallToolRequest, input StockMovementInput) (*mcp.CallToolResult, any, error) {
	input.StockItemName = strings.TrimSpace(input.StockItemName)
	if input.StockItemName == "" {
		return textResult("**Error:** stock_item_name is a required parameter."), nil, nil
	}

	resolved, err := resolveStockItemName(ctx, input.StockItemName)
	if err == nil {
		input.StockItemName = resolved
	}

	staticVars, err := dateRangeStaticVars(input.StartDate, input.EndDate)
	if err != nil {
		return textResult(fmt.Sprintf("Error reading stock movement: %v", err)), nil, nil
	}

	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FM", "FD", "FN", "FT", "FQ").
		Field("FM", "MasterID", "$MasterID").
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FT", "Type", "$VoucherTypeName").
		Field("FQ", "Qty", `$$NumValue:$InventoryEntries[1,@@FilterStockItem].BilledQty`)

	// Use date comparison range filtering.
	if input.StartDate != "" || input.EndDate != "" {
		tdl.CollectionWithFilterAndFetch("Col", tallyxml.CollectionVoucher, "DateFilter", "InventoryEntries").
			Filter("DateFilter", `($Date >= ##SVFROMDATE) and ($Date <= ##SVTODATE)`)
	} else {
		tdl.CollectionWithFetch("Col", tallyxml.CollectionVoucher, "InventoryEntries")
	}
	tdl.FilterFormulae("FilterStockItem", tallyxml.EqualFilter("StockItemName", input.StockItemName))

	root, err := postAndParse(ctx, buildStandardExportQuery(tdl, staticVars))
	if err != nil {
		return textResult(fmt.Sprintf("Error reading stock movement: %v", err)), nil, nil
	}

	from, _ := cleanDateYYYYMMDD(input.StartDate)
	to, _ := cleanDateYYYYMMDD(input.EndDate)

	var inward, outward float64
	var rows []string
	seen := make(map[string]bool)
	for _, row := range root.FindAll("ROW") {
		date := row.FindText("Date")
		if !isInDateRange(date, from, to) {
			continue
		}

		qty := parseFloat(row.FindText("Qty"))
		if qty == 0 {
			continue
		}
		vchNo := row.FindText("VchNo")
		vchType := row.FindText("Type")
		key := row.FindText("MasterID")
		if key == "" || key == "0" {
			key = strings.Join([]string{date, vchNo, vchType, fmt.Sprintf("%.4f", qty)}, "|")
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		inQty, outQty := classifyStockQty(vchType, qty)
		inward += inQty
		outward += outQty
		rows = append(rows, fmt.Sprintf("| %s | %s | %s | %.2f | %.2f |",
			date, emptyDefault(vchNo, "(Auto)"), vchType, inQty, outQty))
	}

	if len(rows) == 0 {
		return textResult(fmt.Sprintf("No stock movement found for item '%s' in the specified period.", input.StockItemName)), nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Stock Movement: %s\n\n", input.StockItemName))
	if input.StartDate != "" || input.EndDate != "" {
		sb.WriteString(fmt.Sprintf("*Date range: %s to %s*\n\n", input.StartDate, input.EndDate))
	}
	sb.WriteString(fmt.Sprintf("**Total Inward:** %.2f\n\n", inward))
	sb.WriteString(fmt.Sprintf("**Total Outward:** %.2f\n\n", outward))
	sb.WriteString(fmt.Sprintf("**Net Movement:** %.2f\n\n", inward-outward))
	sb.WriteString("| Date | Voucher No | Type | Inward Qty | Outward Qty |\n")
	sb.WriteString("| :--- | :--- | :--- | :---: | :---: |\n")
	sb.WriteString(strings.Join(rows, "\n"))

	return textResult(sb.String()), nil, nil
}

// classifyStockQty classifies a voucher type as stock inward or outward movement.
// Only Purchase-type vouchers (and goods receipt notes) count as inward.
// NOTE: Tally's "Receipt" voucher type is a CASH/BANK receipt (money from customer),
// not a goods receipt — do NOT treat it as stock inward.
func classifyStockQty(voucherType string, qty float64) (float64, float64) {
	absQty := math.Abs(qty)
	t := strings.ToLower(voucherType)
	if strings.Contains(t, "sales") || strings.Contains(t, "delivery") {
		return 0, absQty
	}
	// Only "purchase" or "goods receipt note" count as stock inward
	if strings.Contains(t, "purchase") || strings.Contains(t, "goods receipt") {
		return absQty, 0
	}
	// For all other types (Journal, Payment, Receipt, Contra), fall back to sign
	if qty < 0 {
		return 0, absQty
	}
	return absQty, 0
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
