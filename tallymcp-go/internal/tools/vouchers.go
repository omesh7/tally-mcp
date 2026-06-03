package tools

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

// JournalVoucherInput is the input schema for add_quick_journal_voucher.
type JournalVoucherInput struct {
	DebitLedger  string  `json:"debit_ledger" jsonschema:"The ledger to debit (e.g. Factory Rent or Dyeing Charges)"`
	CreditLedger string  `json:"credit_ledger" jsonschema:"The ledger to credit (e.g. HDFC Current Account)"`
	Amount       float64 `json:"amount" jsonschema:"The transaction amount in Rupees"`
	Narration    string  `json:"narration,omitempty" jsonschema:"Brief description / narration of the transaction"`
	Day          int     `json:"day,omitempty" jsonschema:"Day of the month: 1 or 2 (Educational mode limit). Defaults to 1"`
}

// AddQuickJournalVoucher safely records a balanced Journal voucher between two ledgers
// on an Educational Mode safe date (1st or 2nd of April 2026).
func AddQuickJournalVoucher(ctx context.Context, req *mcp.CallToolRequest, input JournalVoucherInput) (*mcp.CallToolResult, any, error) {
	if input.DebitLedger == "" || input.CreditLedger == "" {
		return textResult("❌ **Voucher Posting Failed.**\n- Error: Both debit_ledger and credit_ledger are required fields."), nil, nil
	}
	if input.Amount <= 0 {
		return textResult("❌ **Voucher Posting Failed.**\n- Error: Transaction amount must be greater than zero."), nil, nil
	}

	// Enforce Educational Mode date restriction
	day := input.Day
	if day != 1 && day != 2 {
		day = 1
	}
	dateStr := fmt.Sprintf("2026040%d", day)
	dateDisp := fmt.Sprintf("0%d-Apr-2026", day)

	narr := input.Narration
	if narr == "" {
		narr = fmt.Sprintf("Demo Custom MCP transaction posting on %s.", dateDisp)
	}

	// Build voucher XML using the fluent builder
	voucherXML := tallyxml.NewJournalVoucher(dateStr).
		Debit(input.DebitLedger, input.Amount).
		Credit(input.CreditLedger, input.Amount).
		Narration(narr).
		Build()

	// Wrap in import envelope
	importXML := tallyxml.NewImportEnvelope("All Masters").
		WithData(voucherXML).
		Build()

	root, err := postAndParse(ctx, importXML)
	if err != nil {
		return textResult(fmt.Sprintf("Error posting voucher: %v", err)), nil, nil
	}

	created := root.FindTextDeep("CREATED")
	errors := root.FindTextDeep("ERRORS")

	createdCount, _ := strconv.Atoi(created)
	if createdCount > 0 {
		return textResult(fmt.Sprintf(
			"🎉 **Voucher Posted Successfully!**\n- **Type:** Journal\n- **Date:** %s\n- **Debit:** %s (Rs %s)\n- **Credit:** %s (Rs %s)\n- **Narration:** *%s*",
			dateDisp, input.DebitLedger, formatCurrency(input.Amount),
			input.CreditLedger, formatCurrency(input.Amount), narr,
		)), nil, nil
	}

	return textResult(fmt.Sprintf(
		"❌ **Voucher Posting Failed.**\n- Errors detected by Tally XML: %s",
		errors,
	)), nil, nil
}

// DayBookInput is the input schema for get_day_book.
type DayBookInput struct {
	Date string `json:"date,omitempty" jsonschema:"Optional date in YYYY-MM-DD format to retrieve vouchers for. If omitted, retrieves active period vouchers."`
}

// GetDayBook lists all recorded vouchers in Tally Prime, optionally filtered by date.
func GetDayBook(ctx context.Context, req *mcp.CallToolRequest, input DayBookInput) (*mcp.CallToolResult, any, error) {
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FD", "FT", "FA", "F_NARR").
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FT", "Type", "$VoucherTypeName").
		Field("FA", "Amount", "$$NumValue:$AllLedgerEntries[1].Amount").
		Field("F_NARR", "Narration", "$Narration").
		CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries")

	// Build static vars for date filter if provided
	staticVars := map[string]string{
		"SVCurrentCompany": "##SVCurrentCompany",
		"SVTargetCompany":  "##SVCurrentCompany",
	}
	if input.Date != "" {
		clean := strings.ReplaceAll(input.Date, "-", "")
		if len(clean) == 8 {
			staticVars["SVFROMDATE"] = clean
			staticVars["SVTODATE"] = clean
		}
	}

	xml := buildStandardExportQuery(tdl, staticVars)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error reading Day Book: %v", err)), nil, nil
	}

	var sb strings.Builder
	dateHeader := ""
	if input.Date != "" {
		dateHeader = fmt.Sprintf(" for %s", input.Date)
	}
	sb.WriteString(fmt.Sprintf("### Day Book%s:\n\n", dateHeader))
	sb.WriteString("| Date | Voucher No | Type | Amount (Rs) | Narration |\n")
	sb.WriteString("| :--- | :--- | :--- | :---: | :--- |\n")

	rows := root.FindAll("ROW")
	count := 0
	for _, row := range rows {
		vchNo := row.FindText("VchNo")
		date := row.FindText("Date")
		vchType := row.FindText("Type")
		amt := math.Abs(parseFloat(row.FindText("Amount")))
		narr := row.FindText("Narration")

		if date == "" && vchType == "" && amt == 0 {
			continue
		}

		if vchNo == "" {
			vchNo = "(Auto)"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | *%s* |\n",
			date, vchNo, vchType, formatCurrency(amt), narr))
		count++
	}

	if count == 0 {
		return textResult("No vouchers found in Tally Prime for the specified period."), nil, nil
	}

	return textResult(sb.String()), nil, nil
}
