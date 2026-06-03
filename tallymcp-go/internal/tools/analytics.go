package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

// Month mapping constants
var monthNames = map[string]string{
	"01": "January", "02": "February", "03": "March", "04": "April",
	"05": "May", "06": "June", "07": "July", "08": "August",
	"09": "September", "10": "October", "11": "November", "12": "December",
}

// monthAbbrevToCode converts a 3-letter month abbreviation to a 2-digit code.
func monthAbbrevToCode(abbrev string) string {
	abbrev = strings.TrimSpace(abbrev)
	if len(abbrev) < 3 {
		return ""
	}
	abbrev = strings.ToLower(abbrev[:3])
	for code, name := range monthNames {
		if strings.HasPrefix(strings.ToLower(name), abbrev) {
			return code
		}
	}
	return ""
}

// parseDateMonth extracts the month code from a Tally date string (e.g. "1-Apr-26" → "04").
func parseDateMonth(dateStr string) string {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) >= 8 && dateStr[0] >= '0' && dateStr[0] <= '9' {
		clean := strings.ReplaceAll(strings.ReplaceAll(dateStr, "-", ""), "/", "")
		if len(clean) >= 8 {
			month := clean[4:6]
			if _, ok := monthNames[month]; ok {
				return month
			}
		}
	}
	parts := strings.Split(dateStr, "-")
	if len(parts) >= 2 {
		return monthAbbrevToCode(parts[1])
	}
	parts = strings.Split(dateStr, "/")
	if len(parts) >= 2 && len(parts[1]) == 2 {
		if _, ok := monthNames[parts[1]]; ok {
			return parts[1]
		}
	}
	return ""
}


func monthKeyFromDate(dateStr string) string {
	if t, ok := parseTallyDate(dateStr); ok {
		return t.Format("2006-01")
	}
	if m := parseDateMonth(dateStr); m != "" {
		return "0000-" + m
	}
	return ""
}

func monthLabel(key string) string {
	if len(key) == 7 && key[:4] != "0000" {
		if t, err := time.Parse("2006-01", key); err == nil {
			return t.Format("January 2006")
		}
	}
	if len(key) == 7 {
		if name, ok := monthNames[key[5:7]]; ok {
			return name
		}
	}
	return key
}

func defaultSalesLedgers(ctx context.Context) []string {
	parentMap, err := fetchGroups(ctx)
	if err != nil {
		return nil
	}
	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		return nil
	}
	var sales []string
	for _, l := range ledgers {
		if belongsToGroup(l.Parent, "Sales Accounts", parentMap) {
			sales = append(sales, l.Name)
		}
	}
	if len(sales) == 0 {
		return nil
	}
	sort.Strings(sales)
	return nonEmptyTrimmed(sales)
}

func defaultExpenseLedgers(ctx context.Context) []string {
	parentMap, err := fetchGroups(ctx)
	if err != nil {
		return nil
	}
	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		return nil
	}
	var expenses []string
	for _, l := range ledgers {
		if belongsToGroup(l.Parent, "Direct Expenses", parentMap) ||
			belongsToGroup(l.Parent, "Indirect Expenses", parentMap) ||
			belongsToGroup(l.Parent, "Purchase Accounts", parentMap) {
			expenses = append(expenses, l.Name)
		}
	}
	if len(expenses) == 0 {
		return nil
	}
	sort.Strings(expenses)
	return nonEmptyTrimmed(expenses)
}

// ──────────────────────────────────────────────────────────────────────────────
// get_sales_analytics
// ──────────────────────────────────────────────────────────────────────────────

// SalesAnalyticsInput is the input schema for get_sales_analytics.
type SalesAnalyticsInput struct {
	SalesLedger string `json:"sales_ledger,omitempty" jsonschema:"The sales ledger name to analyze. If omitted, all ledgers under the Sales Accounts group are combined and analyzed."`
	StartDate   string `json:"start_date,omitempty" jsonschema:"Optional start date in YYYY-MM-DD format for analytics filtering."`
	EndDate     string `json:"end_date,omitempty" jsonschema:"Optional end date in YYYY-MM-DD format for analytics filtering."`
}

// GetSalesAnalytics queries Tally for all Sales vouchers, aggregates monthly sales,
// and calculates month-on-month growth.
func GetSalesAnalytics(ctx context.Context, req *mcp.CallToolRequest, input SalesAnalyticsInput) (*mcp.CallToolResult, any, error) {
	var salesLedgerMissing bool
	if strings.TrimSpace(input.SalesLedger) != "" {
		resolved, err := resolveLedgerName(ctx, input.SalesLedger)
		if err == nil {
			input.SalesLedger = resolved
		}
		exists, _ := ledgerExists(ctx, input.SalesLedger)
		if !exists {
			salesLedgerMissing = true
		}
	}
	salesLedgers := []string{input.SalesLedger}
	if strings.TrimSpace(input.SalesLedger) == "" {
		salesLedgers = defaultSalesLedgers(ctx)
	}
	salesFilter := ledgerFilterFormula(salesLedgers)
	if salesFilter == "" {
		return textResult("No sales ledgers could be resolved from Tally's Sales Accounts group. Pass sales_ledger explicitly or check the chart of accounts."), nil, nil
	}

	staticVars, err := dateRangeStaticVars(input.StartDate, input.EndDate)
	if err != nil {
		return textResult(fmt.Sprintf("Error running sales analytics: %v", err)), nil, nil
	}

	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FD", "FA").
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FA", "Amount", `$$NumValue:$AllLedgerEntries[1,@@FilterSalesLedger].Amount`)
	// Use date comparison range filtering.
	// Native SVFROMDATE/SVTODATE scoping alone is not honored in Educational Mode.
	if input.StartDate != "" || input.EndDate != "" {
		tdl.CollectionWithFilterAndFetch("Col", tallyxml.CollectionVoucher, "DateFilter", "AllLedgerEntries").
			Filter("DateFilter", `($Date >= ##SVFROMDATE) and ($Date <= ##SVTODATE)`)
	} else {
		tdl.CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries")
	}
	tdl.FilterFormulae("FilterSalesLedger", salesFilter)

	xml := buildStandardExportQuery(tdl, staticVars)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error running sales analytics: %v", err)), nil, nil
	}

	from, _ := cleanDateYYYYMMDD(input.StartDate)
	to, _ := cleanDateYYYYMMDD(input.EndDate)

	// Monthly aggregation
	monthlySales := map[string]float64{}
	totalSales := 0.0
	txCount := 0

	for _, row := range root.FindAll("ROW") {
		dateStr := row.FindText("Date")
		if !isInDateRange(dateStr, from, to) {
			continue
		}
		amt := math.Abs(parseFloat(row.FindText("Amount")))
		if amt == 0 || dateStr == "" {
			continue
		}

		mCode := monthKeyFromDate(dateStr)
		if mCode != "" {
			monthlySales[mCode] += amt
			totalSales += amt
			txCount++
		}
	}

	// Build output with MoM growth
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Monthly Sales & Growth Analytics (%s):\n\n", strings.Join(salesLedgers, ", ")))
	if input.StartDate != "" || input.EndDate != "" {
		sb.WriteString(fmt.Sprintf("*Date range: %s to %s*\n\n", input.StartDate, input.EndDate))
	}
	if salesLedgerMissing {
		sb.WriteString(fmt.Sprintf("> [!WARNING]\n> Sales ledger '%s' was not found in Tally's chart of accounts.\n\n", input.SalesLedger))
	}
	if txCount == 0 {
		sb.WriteString("> [!WARNING]\n> No matching sales vouchers were found. Check the ledger name and date range before using this as a zero-sales result.\n\n")
	}
	sb.WriteString("| Month | Total Sales (Rs) | Month-on-Month (MoM) Growth |\n")
	sb.WriteString("| :--- | :---: | :---: |\n")

	months := sortedMonthKeys(monthlySales)
	var prevSales *float64

	for _, m := range months {
		sales := monthlySales[m]
		name := monthLabel(m)
		growth := "N/A (Baseline)"

		if prevSales != nil && *prevSales > 0 {
			g := ((sales - *prevSales) / *prevSales) * 100
			growth = fmt.Sprintf("**%+.1f%%**", g)
		} else if prevSales != nil && sales > 0 {
			growth = "New Sales"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, formatCurrency(sales), growth))
		s := sales
		prevSales = &s
	}
	sb.WriteString(fmt.Sprintf("| **TOTAL REVENUE** | **%s** | *%d Transactions* |\n", formatCurrency(totalSales), txCount))

	return textResult(sb.String()), nil, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// get_expense_analytics
// ──────────────────────────────────────────────────────────────────────────────

// ExpenseAnalyticsInput is the input schema for get_expense_analytics.
type ExpenseAnalyticsInput struct {
	SalesLedger    string   `json:"sales_ledger,omitempty" jsonschema:"The sales ledger name to compare profit margins against. If omitted, sales ledgers are auto-detected from Sales Accounts."`
	ExpenseLedgers []string `json:"expense_ledgers,omitempty" jsonschema:"List of expense ledger names to analyze. If empty, expense ledgers are auto-detected from Direct Expenses, Indirect Expenses, and Purchase Accounts."`
	StartDate      string   `json:"start_date,omitempty" jsonschema:"Optional start date in YYYY-MM-DD format for analytics filtering."`
	EndDate        string   `json:"end_date,omitempty" jsonschema:"Optional end date in YYYY-MM-DD format for analytics filtering."`
}

// GetExpenseAnalytics fetches multi-period expenses and sales to calculate
// net profit margins and profitability trends.
func GetExpenseAnalytics(ctx context.Context, req *mcp.CallToolRequest, input ExpenseAnalyticsInput) (*mcp.CallToolResult, any, error) {
	var salesLedgerMissing bool
	if strings.TrimSpace(input.SalesLedger) != "" {
		resolved, err := resolveLedgerName(ctx, input.SalesLedger)
		if err == nil {
			input.SalesLedger = resolved
		}
		exists, _ := ledgerExists(ctx, input.SalesLedger)
		if !exists {
			salesLedgerMissing = true
		}
	}
	salesLedgers := []string{input.SalesLedger}
	if strings.TrimSpace(input.SalesLedger) == "" {
		salesLedgers = defaultSalesLedgers(ctx)
	}
	salesFilter := ledgerFilterFormula(salesLedgers)
	if salesFilter == "" {
		return textResult("No sales ledgers could be resolved from Tally's Sales Accounts group. Pass sales_ledger explicitly or check the chart of accounts."), nil, nil
	}

	var missingExpenses []string
	expenseLedgers := input.ExpenseLedgers
	if len(expenseLedgers) > 0 {
		for i, exp := range expenseLedgers {
			resolved, err := resolveLedgerName(ctx, exp)
			if err == nil {
				expenseLedgers[i] = resolved
			}
			exists, _ := ledgerExists(ctx, expenseLedgers[i])
			if !exists {
				missingExpenses = append(missingExpenses, expenseLedgers[i])
			}
		}
	} else {
		expenseLedgers = defaultExpenseLedgers(ctx)
	}
	expenseFilter := ledgerFilterFormula(expenseLedgers)
	if expenseFilter == "" {
		return textResult("No expense ledgers could be resolved from Tally's Direct Expenses, Indirect Expenses, or Purchase Accounts groups. Pass expense_ledgers explicitly or check the chart of accounts."), nil, nil
	}

	staticVars, err := dateRangeStaticVars(input.StartDate, input.EndDate)
	if err != nil {
		return textResult(fmt.Sprintf("Error running expense analytics: %v", err)), nil, nil
	}

	expTDL := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FD", "FA").
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FA", "Amount", `$$NumValue:$AllLedgerEntries[1,@@FilterExpenseLedger].Amount`)
	// Use date comparison range filtering.
	if input.StartDate != "" || input.EndDate != "" {
		expTDL.CollectionWithFilterAndFetch("Col", tallyxml.CollectionVoucher, "DateFilter", "AllLedgerEntries").
			Filter("DateFilter", `($Date >= ##SVFROMDATE) and ($Date <= ##SVTODATE)`)
	} else {
		expTDL.CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries")
	}
	expTDL.FilterFormulae("FilterExpenseLedger", expenseFilter)

	expXML := buildStandardExportQuery(expTDL, staticVars)

	// 2. Build sales query
	saleTDL := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FD", "FA").
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FA", "Amount", `$$NumValue:$AllLedgerEntries[1,@@FilterSalesLedger3].Amount`)
	// Use date comparison range filtering.
	if input.StartDate != "" || input.EndDate != "" {
		saleTDL.CollectionWithFilterAndFetch("Col", tallyxml.CollectionVoucher, "DateFilter", "AllLedgerEntries").
			Filter("DateFilter", `($Date >= ##SVFROMDATE) and ($Date <= ##SVTODATE)`)
	} else {
		saleTDL.CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries")
	}
	saleTDL.FilterFormulae("FilterSalesLedger3", salesFilter)

	saleXML := buildStandardExportQuery(saleTDL, staticVars)

	// 3. Execute both queries (serialized through mutex)
	expRoot, err := postAndParse(ctx, expXML)
	if err != nil {
		return textResult(fmt.Sprintf("Error running expense analytics: %v", err)), nil, nil
	}

	saleRoot, err := postAndParse(ctx, saleXML)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching sales for profitability: %v", err)), nil, nil
	}

	from, _ := cleanDateYYYYMMDD(input.StartDate)
	to, _ := cleanDateYYYYMMDD(input.EndDate)

	// 4. Aggregate monthly
	monthlyExpenses := map[string]float64{}
	monthlySales := map[string]float64{}

	var totalExpensesTxCount, totalSalesTxCount int
	for _, row := range expRoot.FindAll("ROW") {
		dateStr := row.FindText("Date")
		if !isInDateRange(dateStr, from, to) {
			continue
		}
		amt := math.Abs(parseFloat(row.FindText("Amount")))
		if amt == 0 || dateStr == "" {
			continue
		}
		if mCode := monthKeyFromDate(dateStr); mCode != "" {
			monthlyExpenses[mCode] += amt
			totalExpensesTxCount++
		}
	}

	for _, row := range saleRoot.FindAll("ROW") {
		dateStr := row.FindText("Date")
		if !isInDateRange(dateStr, from, to) {
			continue
		}
		amt := math.Abs(parseFloat(row.FindText("Amount")))
		if amt == 0 || dateStr == "" {
			continue
		}
		if mCode := monthKeyFromDate(dateStr); mCode != "" {
			monthlySales[mCode] += amt
			totalSalesTxCount++
		}
	}

	// 5. Build profitability report
	var sb strings.Builder
	sb.WriteString("### Monthly Profitability Trends:\n\n")
	sb.WriteString(fmt.Sprintf("*Sales ledgers: %s*\n\n", strings.Join(salesLedgers, ", ")))
	sb.WriteString(fmt.Sprintf("*Expense ledgers: %s*\n\n", strings.Join(expenseLedgers, ", ")))
	if input.StartDate != "" || input.EndDate != "" {
		sb.WriteString(fmt.Sprintf("*Date range: %s to %s*\n\n", input.StartDate, input.EndDate))
	}
	if salesLedgerMissing {
		sb.WriteString(fmt.Sprintf("> [!WARNING]\n> Sales ledger '%s' was not found in Tally's chart of accounts.\n\n", input.SalesLedger))
	}
	if len(missingExpenses) > 0 {
		sb.WriteString(fmt.Sprintf("> [!WARNING]\n> Expense ledger(s) not found in Tally: %s. Check ledger names before using this as a zero-expense result.\n\n", strings.Join(missingExpenses, ", ")))
	}
	if totalSalesTxCount == 0 {
		sb.WriteString("> [!WARNING]\n> No matching sales vouchers were found for the specified sales ledgers. Check ledger names and date range before using this as a zero-sales result.\n\n")
	}
	if totalExpensesTxCount == 0 {
		sb.WriteString("> [!WARNING]\n> No matching expense vouchers were found for the specified expense ledgers. Check ledger names and date range before using this as a zero-expense result.\n\n")
	}
	sb.WriteString("| Month | Total Sales (Rs) | Total Expenses (Rs) | Net Profit (Rs) | Net Margin |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")

	months := sortedMonthKeys(monthlySales, monthlyExpenses)
	var totalS, totalE float64

	for _, m := range months {
		s := monthlySales[m]
		e := monthlyExpenses[m]
		p := s - e
		margin := 0.0
		if s > 0 {
			margin = (p / s) * 100
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | **%+.1f%%** |\n",
			monthLabel(m), formatCurrency(s), formatCurrency(e), formatCurrency(p), margin))
		totalS += s
		totalE += e
	}

	totalP := totalS - totalE
	totalMargin := 0.0
	if totalS > 0 {
		totalMargin = (totalP / totalS) * 100
	}
	sb.WriteString(fmt.Sprintf("| **CUMULATIVE** | **%s** | **%s** | **%s** | **%+.1f%%** |\n",
		formatCurrency(totalS), formatCurrency(totalE), formatCurrency(totalP), totalMargin))

	return textResult(sb.String()), nil, nil
}

// sortedMonthKeys returns unique month keys sorted chronologically.
func sortedMonthKeys(monthMaps ...map[string]float64) []string {
	seen := make(map[string]bool)
	for _, m := range monthMaps {
		for k := range m {
			if k != "" {
				seen[k] = true
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
