package tools

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

const maxJournalVoucherAmount = 10000000.0

// JournalVoucherInput is the input schema for add_quick_journal_voucher.
type JournalVoucherInput struct {
	DebitLedger  string  `json:"debit_ledger" jsonschema:"The ledger to debit"`
	CreditLedger string  `json:"credit_ledger" jsonschema:"The ledger to credit"`
	Amount       float64 `json:"amount" jsonschema:"The transaction amount in Rupees"`
	Narration    string  `json:"narration,omitempty" jsonschema:"Brief description / narration of the transaction"`
	Date         string  `json:"date,omitempty" jsonschema:"Optional voucher date in YYYY-MM-DD format. If omitted, day is used for Educational Mode compatibility."`
	Day          int     `json:"day,omitempty" jsonschema:"Day of the month: 1 or 2 when date is omitted. Defaults to 1"`
}

// AddQuickJournalVoucher records a balanced Journal voucher between two ledgers.
func AddQuickJournalVoucher(ctx context.Context, req *mcp.CallToolRequest, input JournalVoucherInput) (*mcp.CallToolResult, any, error) {
	input.DebitLedger = strings.TrimSpace(input.DebitLedger)
	input.CreditLedger = strings.TrimSpace(input.CreditLedger)

	if input.DebitLedger == "" || input.CreditLedger == "" {
		return textResult("**Voucher Posting Failed.**\n- Error: Both debit_ledger and credit_ledger are required fields."), nil, nil
	}

	if resolved, err := resolveLedgerName(ctx, input.DebitLedger); err == nil {
		input.DebitLedger = resolved
	}
	if resolved, err := resolveLedgerName(ctx, input.CreditLedger); err == nil {
		input.CreditLedger = resolved
	}
	if sameName(input.DebitLedger, input.CreditLedger) {
		return textResult("**Voucher Posting Failed.**\n- Error: debit_ledger and credit_ledger must be different ledgers."), nil, nil
	}
	if input.Amount <= 0 {
		return textResult("**Voucher Posting Failed.**\n- Error: Transaction amount must be greater than zero."), nil, nil
	}
	if input.Amount > maxJournalVoucherAmount {
		return textResult(fmt.Sprintf("**Voucher Posting Failed.**\n- Error: Transaction amount exceeds the MCP safety limit of Rs %s.", formatCurrency(maxJournalVoucherAmount))), nil, nil
	}

	dateStr, dateDisp, err := journalVoucherDate(input)
	if err != nil {
		return textResult(fmt.Sprintf("**Voucher Posting Failed.**\n- Error: %v", err)), nil, nil
	}

	narr := strings.TrimSpace(input.Narration)
	if narr == "" {
		narr = fmt.Sprintf("MCP journal voucher posted on %s.", dateDisp)
	}

	voucherXML := tallyxml.NewJournalVoucher(dateStr).
		Debit(input.DebitLedger, input.Amount).
		Credit(input.CreditLedger, input.Amount).
		Narration(narr).
		Build()

	importXML := tallyxml.NewImportEnvelope("All Masters").
		WithData(voucherXML).
		Build()

	root, err := postAndParse(ctx, importXML)
	if err != nil {
		return textResult(fmt.Sprintf("Error posting voucher: %v", err)), nil, nil
	}

	created := root.FindTextDeep("CREATED")
	errors := root.FindTextDeep("ERRORS")
	lineError := root.FindTextDeep("LINEERROR")
	exceptions := root.FindTextDeep("EXCEPTIONS")
	cancelled := root.FindTextDeep("CANCELLED")

	createdCount, _ := strconv.Atoi(created)
	if createdCount > 0 {
		return textResult(fmt.Sprintf(
			"**Voucher Posted Successfully.**\n- **Type:** Journal\n- **Date:** %s\n- **Debit:** %s (Rs %s)\n- **Credit:** %s (Rs %s)\n- **Narration:** *%s*",
			dateDisp, input.DebitLedger, formatCurrency(input.Amount),
			input.CreditLedger, formatCurrency(input.Amount), narr,
		)), nil, nil
	}

	var details []string
	if lineError != "" {
		details = append(details, "Tally line error: "+lineError)
	}
	if exceptions != "" && exceptions != "0" {
		details = append(details, "Exceptions: "+exceptions)
	}
	if errors != "" && errors != "0" {
		details = append(details, "Errors: "+errors)
	}
	if cancelled != "" && cancelled != "0" {
		details = append(details, "Cancelled: "+cancelled)
	}
	if len(details) == 0 {
		details = append(details, fmt.Sprintf("Tally did not create the voucher. CREATED=%s, ERRORS=%s.", created, errors))
	}

	return textResult("**Voucher Posting Failed.**\n- " + strings.Join(details, "\n- ")), nil, nil
}

func journalVoucherDate(input JournalVoucherInput) (string, string, error) {
	if strings.TrimSpace(input.Date) != "" {
		clean, err := cleanDateYYYYMMDD(input.Date)
		if err != nil {
			return "", "", err
		}
		return clean, input.Date, nil
	}

	day := input.Day
	if day == 0 {
		day = 1
	}
	if day != 1 && day != 2 {
		return "", "", fmt.Errorf("day must be 1 or 2 when date is omitted. Pass date for a specific voucher date")
	}
	// Use current fiscal year dynamically: Indian fiscal year starts April.
	// Educational Mode only allows 1st or 2nd of any month.
	now := time.Now()
	fiscalYear := now.Year()
	if now.Month() < time.April {
		fiscalYear-- // Jan-Mar belongs to previous fiscal year
	}
	dateStr := fmt.Sprintf("%d04%02d", fiscalYear, day)
	dateDisp := fmt.Sprintf("%02d-Apr-%d", day, fiscalYear)
	return dateStr, dateDisp, nil
}

// DayBookInput is the input schema for get_day_book.
type DayBookInput struct {
	Date      string `json:"date,omitempty" jsonschema:"Optional single date in YYYY-MM-DD format to retrieve vouchers for."`
	StartDate string `json:"start_date,omitempty" jsonschema:"Optional start date in YYYY-MM-DD format."`
	EndDate   string `json:"end_date,omitempty" jsonschema:"Optional end date in YYYY-MM-DD format."`
}

// GetDayBook lists recorded vouchers in Tally Prime, optionally filtered by date or date range.
func GetDayBook(ctx context.Context, req *mcp.CallToolRequest, input DayBookInput) (*mcp.CallToolResult, any, error) {
	startDate := input.StartDate
	endDate := input.EndDate
	if input.Date != "" {
		startDate = input.Date
		endDate = input.Date
	}

	staticVars, err := dateRangeStaticVars(startDate, endDate)
	if err != nil {
		return textResult(fmt.Sprintf("Error reading Day Book: %v", err)), nil, nil
	}

	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FM", "FN", "FD", "FT", "FA", "F_NARR").
		Field("FM", "MasterID", "$MasterID").
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FT", "Type", "$VoucherTypeName").
		Field("FA", "Amount", "$$NumValue:$AllLedgerEntries[1].Amount").
		Field("F_NARR", "Narration", "$Narration")

	// When date range provided, filter using Tally's IsBetween TDL function.
	// The SVFROMDATE/SVTODATE static vars scope is honored inside the filter.
	// Without a filter, Tally returns all vouchers regardless of static vars
	// in Educational Mode.
	if startDate != "" || endDate != "" {
		tdl.CollectionWithFilterAndFetch("Col", tallyxml.CollectionVoucher, "DateFilter", "AllLedgerEntries").
			Filter("DateFilter", `($Date >= ##SVFROMDATE) and ($Date <= ##SVTODATE)`)
	} else {
		tdl.CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries")
	}

	xml := buildStandardExportQuery(tdl, staticVars)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error reading Day Book: %v", err)), nil, nil
	}

	var sb strings.Builder
	dateHeader := ""
	if startDate != "" || endDate != "" {
		dateHeader = fmt.Sprintf(" for %s to %s", startDate, endDate)
	}
	sb.WriteString(fmt.Sprintf("### Day Book%s:\n\n", dateHeader))
	sb.WriteString("| Date | Voucher No | Type | Amount (Rs) | Narration |\n")
	sb.WriteString("| :--- | :--- | :--- | :---: | :--- |\n")

	from, _ := cleanDateYYYYMMDD(startDate)
	to, _ := cleanDateYYYYMMDD(endDate)

	rows := root.FindAll("ROW")
	count := 0
	seen := make(map[string]bool)
	for _, row := range rows {
		date := row.FindText("Date")
		if !isInDateRange(date, from, to) {
			continue
		}

		masterID := row.FindText("MasterID")
		vchNo := row.FindText("VchNo")
		vchType := row.FindText("Type")
		amt := math.Abs(parseFloat(row.FindText("Amount")))
		narr := strings.TrimSpace(row.FindText("Narration"))

		if date == "" && vchType == "" && amt == 0 {
			continue
		}
		if vchNo == "" {
			vchNo = "(Auto)"
		}
		if narr == "" || narr == "**" {
			narr = "no narration"
		}

		key := masterID
		if key == "" || key == "0" {
			key = strings.Join([]string{date, vchNo, vchType, fmt.Sprintf("%.2f", amt), narr}, "|")
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | *%s* |\n",
			date, vchNo, vchType, formatCurrency(amt), narr))
		count++
	}

	if count == 0 {
		return textResult("No vouchers found in Tally Prime for the specified period."), nil, nil
	}

	return textResult(sb.String()), nil, nil
}
