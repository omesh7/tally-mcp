package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tallymcp/internal/xml"
)

// Month mapping constants
var monthNames = map[string]string{
	"04": "April", "05": "May", "06": "June", "07": "July", "08": "August",
}

// monthAbbrevToCode converts a 3-letter month abbreviation to a 2-digit code.
func monthAbbrevToCode(abbrev string) string {
	abbrev = strings.Title(strings.ToLower(abbrev[:3]))
	for code, name := range monthNames {
		if strings.HasPrefix(name, abbrev) {
			return code
		}
	}
	return ""
}

// parseDateMonth extracts the month code from a Tally date string (e.g. "1-Apr-26" → "04").
func parseDateMonth(dateStr string) string {
	parts := strings.Split(dateStr, "-")
	if len(parts) >= 2 {
		return monthAbbrevToCode(parts[1])
	}
	return ""
}

// ──────────────────────────────────────────────────────────────────────────────
// get_sales_analytics
// ──────────────────────────────────────────────────────────────────────────────

// SalesAnalyticsInput is the input schema for get_sales_analytics.
type SalesAnalyticsInput struct {
	SalesLedger string `json:"sales_ledger,omitempty" jsonschema:"The sales ledger name to analyze (e.g. 'Sales' or 'Garment Sales'). Defaults to 'Sales'"`
}

// GetSalesAnalytics queries Tally for all Sales vouchers, aggregates monthly sales,
// and calculates month-on-month growth.
func GetSalesAnalytics(ctx context.Context, req *mcp.CallToolRequest, input SalesAnalyticsInput) (*mcp.CallToolResult, any, error) {
	salesLedger := input.SalesLedger
	if salesLedger == "" {
		salesLedger = "Sales"
	}

	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FD", "FA").
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FA", "Amount", `$$NumValue:$AllLedgerEntries[1,@@FilterSalesLedger].Amount`).
		CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries").
		FilterFormulae("FilterSalesLedger", tallyxml.EqualFilter("LedgerName", salesLedger))

	xml := buildStandardExportQuery(tdl, nil)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error running sales analytics: %v", err)), nil, nil
	}

	// Monthly aggregation
	monthlySales := map[string]float64{"04": 0, "05": 0, "06": 0, "07": 0, "08": 0}
	totalSales := 0.0
	txCount := 0

	for _, row := range root.FindAll("ROW") {
		dateStr := row.FindText("Date")
		amt := parseFloat(row.FindText("Amount"))
		if amt == 0 || dateStr == "" {
			continue
		}

		mCode := parseDateMonth(dateStr)
		if _, ok := monthlySales[mCode]; ok {
			monthlySales[mCode] += amt
			totalSales += amt
			txCount++
		}
	}

	// Build output with MoM growth
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Monthly Sales & Growth Analytics (%s):\n\n", salesLedger))
	sb.WriteString("| Month | Total Sales (Rs) | Month-on-Month (MoM) Growth |\n")
	sb.WriteString("| :--- | :---: | :---: |\n")

	months := sortedMonthKeys(monthlySales)
	var prevSales *float64

	for _, m := range months {
		sales := monthlySales[m]
		name := monthNames[m]
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
	SalesLedger    string   `json:"sales_ledger,omitempty" jsonschema:"The sales ledger name to compare profit margins against. Defaults to 'Sales'"`
	ExpenseLedgers []string `json:"expense_ledgers,omitempty" jsonschema:"List of expense ledger names to analyze (e.g. ['Rent', 'Salaries']). If empty, it defaults to standard categories."`
}

// GetExpenseAnalytics fetches multi-period expenses and sales to calculate
// net profit margins and profitability trends.
func GetExpenseAnalytics(ctx context.Context, req *mcp.CallToolRequest, input ExpenseAnalyticsInput) (*mcp.CallToolResult, any, error) {
	salesLedger := input.SalesLedger
	if salesLedger == "" {
		salesLedger = "Sales"
	}

	var expenseFilter string
	if len(input.ExpenseLedgers) > 0 {
		var filters []string
		for _, ledger := range input.ExpenseLedgers {
			filters = append(filters, tallyxml.EqualFilter("LedgerName", ledger))
		}
		expenseFilter = strings.Join(filters, " or ")
	} else {
		// Default fallback categories
		expenseFilter = strings.Join([]string{
			`$$IsEqual:$LedgerName:"Direct Expenses"`,
			`$$IsEqual:$LedgerName:"Indirect Expenses"`,
			`$$IsEqual:$LedgerName:"Fabric Purchases"`,
			`$$IsEqual:$LedgerName:"Factory Rent"`,
			`$$IsEqual:$LedgerName:"Electricity"`,
			`$$IsEqual:$LedgerName:"Salaries"`,
			`$$IsEqual:$LedgerName:"Labour Stitching"`,
			`$$IsEqual:$LedgerName:"Transport Freight"`,
			`$$IsEqual:$LedgerName:"Marketing Ads"`,
			`$$IsEqual:$LedgerName:"Dyeing Charges"`,
		}, " or ")
	}

	expTDL := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FD", "FA").
		Field("FN", "VchNo", tallyxml.FormulaVoucherNumber).
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FA", "Amount", `$$NumValue:$AllLedgerEntries[1,@@FilterExpenseLedger].Amount`).
		CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries").
		FilterFormulae("FilterExpenseLedger", expenseFilter)

	expXML := buildStandardExportQuery(expTDL, nil)

	// 2. Build sales query
	saleTDL := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FD", "FA").
		Field("FD", "Date", tallyxml.FormulaDate).
		Field("FA", "Amount", `$$NumValue:$AllLedgerEntries[1,@@FilterSalesLedger3].Amount`).
		CollectionWithFetch("Col", tallyxml.CollectionVoucher, "AllLedgerEntries").
		FilterFormulae("FilterSalesLedger3", tallyxml.EqualFilter("LedgerName", salesLedger))

	saleXML := buildStandardExportQuery(saleTDL, nil)

	// 3. Execute both queries (serialized through mutex)
	expRoot, err := postAndParse(ctx, expXML)
	if err != nil {
		return textResult(fmt.Sprintf("Error running expense analytics: %v", err)), nil, nil
	}

	saleRoot, err := postAndParse(ctx, saleXML)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching sales for profitability: %v", err)), nil, nil
	}

	// 4. Aggregate monthly
	monthlyExpenses := map[string]float64{"04": 0, "05": 0, "06": 0, "07": 0, "08": 0}
	monthlySales := map[string]float64{"04": 0, "05": 0, "06": 0, "07": 0, "08": 0}

	for _, row := range expRoot.FindAll("ROW") {
		dateStr := row.FindText("Date")
		amt := parseFloat(row.FindText("Amount"))
		if amt == 0 || dateStr == "" {
			continue
		}
		if mCode := parseDateMonth(dateStr); mCode != "" {
			if _, ok := monthlyExpenses[mCode]; ok {
				monthlyExpenses[mCode] += amt
			}
		}
	}

	for _, row := range saleRoot.FindAll("ROW") {
		dateStr := row.FindText("Date")
		amt := parseFloat(row.FindText("Amount"))
		if amt == 0 || dateStr == "" {
			continue
		}
		if mCode := parseDateMonth(dateStr); mCode != "" {
			if _, ok := monthlySales[mCode]; ok {
				monthlySales[mCode] += amt
			}
		}
	}

	// 5. Build profitability report
	var sb strings.Builder
	sb.WriteString("### Monthly Profitability Trends:\n\n")
	sb.WriteString("| Month | Total Sales (Rs) | Total Expenses (Rs) | Net Profit (Rs) | Net Margin |\n")
	sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")

	months := sortedMonthKeys(monthlySales)
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
			monthNames[m], formatCurrency(s), formatCurrency(e), formatCurrency(p), margin))
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

// sortedMonthKeys returns map keys sorted alphabetically.
func sortedMonthKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
