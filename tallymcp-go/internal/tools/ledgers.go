package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
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
	var pnlBal float64
	var pnlType string

	for _, row := range root.FindAll("ROW") {
		name := row.FindText("Name")
		parent := row.FindText("Parent")
		bal := math.Abs(parseFloat(row.FindText("Balance")))
		btype := row.FindText("Type")

		if bal == 0 {
			continue
		}

		if strings.EqualFold(name, "Profit & Loss A/c") || strings.EqualFold(name, "Profit and Loss A/c") {
			pnlBal = bal
			pnlType = btype
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
	sb.WriteString(fmt.Sprintf("| **TOTAL** | | **%s** | **%s** |\n\n", formatCurrency(totalDr), formatCurrency(totalCr)))
	if pnlBal > 0 {
		label := "Profit"
		if pnlType == "Dr" || pnlType == "Debit" {
			label = "Loss"
		}
		sb.WriteString(fmt.Sprintf("**Net Profit / (Loss):** Rs %s (%s)\n\n", formatCurrency(pnlBal), label))
	}
	sb.WriteString("> [!NOTE]\n> Zero-balance ledgers are excluded from the Trial Balance.\n")

	return textResult(sb.String()), nil, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// get_ledger_closing_balance
// ──────────────────────────────────────────────────────────────────────────────

// LedgerBalanceInput is the input schema for get_ledger_closing_balance.
type LedgerBalanceInput struct {
	LedgerName string `json:"ledger_name" jsonschema:"The exact name of the ledger"`
	Date       string `json:"date,omitempty" jsonschema:"Optional date in YYYY-MM-DD format to retrieve balance as of that date"`
}

// GetLedgerClosingBalance retrieves the closing balance of a specific ledger account.
func GetLedgerClosingBalance(ctx context.Context, req *mcp.CallToolRequest, input LedgerBalanceInput) (*mcp.CallToolResult, any, error) {
	if input.LedgerName == "" {
		return textResult("❌ **Error:** ledger_name is a required parameter."), nil, nil
	}

	resolved, err := resolveLedgerName(ctx, input.LedgerName)
	if err == nil {
		input.LedgerName = resolved
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
		clean, err := cleanDateYYYYMMDD(input.Date)
		if err != nil {
			return textResult(fmt.Sprintf("❌ **Error:** %v", err)), nil, nil
		}
		tTo, ok := parseTallyDate(clean)
		if !ok {
			return textResult("❌ **Error:** failed to parse target date internally"), nil, nil
		}
		staticVars["SVFROMDATE"] = "01-Jan-1900"
		staticVars["SVTODATE"] = tTo.Format("02-Jan-2006")
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

// ──────────────────────────────────────────────────────────────────────────────
// Helpers for Hierarchy and Dynamic Balances
// ──────────────────────────────────────────────────────────────────────────────

// GroupInfo represents basic Tally account group hierarchy information.
type GroupInfo struct {
	Name   string
	Parent string
}

// LedgerInfo represents closing and opening balance information for a Tally ledger.
type LedgerInfo struct {
	Name           string
	Parent         string
	OpeningBalance float64
	OpeningType    string // "Dr" or "Cr"
	ClosingBalance float64
	ClosingType    string // "Dr" or "Cr"
}

// fetchGroups retrieves all account groups and their parents from Tally.
func fetchGroups(ctx context.Context) (map[string]string, error) {
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FP").
		Field("FN", "Name", tallyxml.FormulaName).
		Field("FP", "Parent", tallyxml.FormulaParent).
		Collection("Col", "Group")

	xml := buildStandardExportQuery(tdl, nil)
	root, err := postAndParse(ctx, xml)
	if err != nil {
		return nil, err
	}

	parentMap := make(map[string]string)
	for _, row := range root.FindAll("ROW") {
		name := row.FindText("Name")
		parent := row.FindText("Parent")
		if name != "" {
			parentMap[name] = parent
		}
	}
	return parentMap, nil
}

// fetchLedgers retrieves closing and opening balance data for all ledgers.
func fetchLedgers(ctx context.Context) ([]LedgerInfo, error) {
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FP", "F_OP_BAL", "F_OP_TYPE", "F_CL_BAL", "F_CL_TYPE").
		Field("FN", "Name", tallyxml.FormulaName).
		Field("FP", "Parent", tallyxml.FormulaParent).
		Field("F_OP_BAL", "OpeningBalance", `$$NumValue:$OpeningBalance`).
		Field("F_OP_TYPE", "OpeningType", `if $$IsDebit:$OpeningBalance then "Dr" else "Cr"`).
		Field("F_CL_BAL", "ClosingBalance", tallyxml.FormulaClosingBalance).
		Field("F_CL_TYPE", "ClosingType", tallyxml.FormulaIsDebit).
		Collection("Col", tallyxml.CollectionLedger)

	xml := buildStandardExportQuery(tdl, nil)
	root, err := postAndParse(ctx, xml)
	if err != nil {
		return nil, err
	}

	var ledgers []LedgerInfo
	for _, row := range root.FindAll("ROW") {
		name := row.FindText("Name")
		if name == "" {
			continue
		}
		ledgers = append(ledgers, LedgerInfo{
			Name:           name,
			Parent:         row.FindText("Parent"),
			OpeningBalance: math.Abs(parseFloat(row.FindText("OpeningBalance"))),
			OpeningType:    row.FindText("OpeningType"),
			ClosingBalance: math.Abs(parseFloat(row.FindText("ClosingBalance"))),
			ClosingType:    row.FindText("ClosingType"),
		})
	}
	return ledgers, nil
}

// belongsToGroup checks if a group (or any of its parents) resolves to targetGroup.
func belongsToGroup(groupName, targetGroup string, parentMap map[string]string) bool {
	current := groupName
	visited := make(map[string]bool)
	for current != "" {
		if strings.EqualFold(current, targetGroup) {
			return true
		}
		if visited[current] {
			break
		}
		visited[current] = true
		parent, exists := parentMap[current]
		if !exists || parent == "" {
			break
		}
		current = parent
	}
	return false
}

// ──────────────────────────────────────────────────────────────────────────────
// get_outstanding_receivables
// ──────────────────────────────────────────────────────────────────────────────

// OutstandingReceivablesInput is the input schema for get_outstanding_receivables (no parameters).
type OutstandingReceivablesInput struct{}

// GetOutstandingReceivables lists all customer ledger balances under 'Sundry Debtors' sorted descending.
func GetOutstandingReceivables(ctx context.Context, req *mcp.CallToolRequest, input OutstandingReceivablesInput) (*mcp.CallToolResult, any, error) {
	parentMap, err := fetchGroups(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching group hierarchy: %v", err)), nil, nil
	}

	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching ledger balances: %v", err)), nil, nil
	}

	type debtorItem struct {
		Name    string
		Group   string
		Balance float64
		Type    string // "Dr" or "Cr"
	}

	var debtors []debtorItem
	var creditDebtors []debtorItem
	for _, l := range ledgers {
		if belongsToGroup(l.Parent, "Sundry Debtors", parentMap) {
			if l.ClosingBalance > 0 {
				item := debtorItem{
					Name:    l.Name,
					Group:   l.Parent,
					Balance: l.ClosingBalance,
					Type:    l.ClosingType,
				}
				if l.ClosingType == "Dr" {
					debtors = append(debtors, item)
				} else {
					creditDebtors = append(creditDebtors, item)
				}
			}
		}
	}

	if len(debtors) == 0 && len(creditDebtors) == 0 {
		return textResult("### Outstanding Receivables (Sundry Debtors)\n\nNo outstanding receivables or customer credit balances found."), nil, nil
	}

	// Sort descending by balance
	sort.Slice(debtors, func(i, j int) bool {
		return debtors[i].Balance > debtors[j].Balance
	})
	sort.Slice(creditDebtors, func(i, j int) bool {
		return creditDebtors[i].Balance > creditDebtors[j].Balance
	})

	var sb strings.Builder
	sb.WriteString("### Outstanding Receivables (Sundry Debtors)\n\n")

	if len(debtors) > 0 {
		sb.WriteString("| Customer Ledger Name | Group / Parent | Outstanding Balance (Rs) | Type |\n")
		sb.WriteString("| :--- | :--- | :---: | :---: |\n")

		var total float64
		for _, d := range debtors {
			sb.WriteString(fmt.Sprintf("| %s | *%s* | %s | %s |\n", d.Name, d.Group, formatCurrency(d.Balance), d.Type))
			total += d.Balance
		}
		sb.WriteString(fmt.Sprintf("| **TOTAL OUTSTANDING** | | **%s** | **Dr** |\n\n", formatCurrency(total)))
	} else {
		sb.WriteString("No active debit outstanding balances found.\n\n")
	}

	if len(creditDebtors) > 0 {
		sb.WriteString("### Credit Balances in Debtors (Customer Credits / Prepayments)\n\n")
		sb.WriteString("| Customer Ledger Name | Group / Parent | Credit Balance (Rs) |\n")
		sb.WriteString("| :--- | :--- | :---: |\n")
		for _, cd := range creditDebtors {
			sb.WriteString(fmt.Sprintf("| %s | *%s* | %s |\n", cd.Name, cd.Group, formatCurrency(cd.Balance)))
		}
	}

	return textResult(sb.String()), nil, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// get_outstanding_payables
// ──────────────────────────────────────────────────────────────────────────────

// OutstandingPayablesInput is the input schema for get_outstanding_payables (no parameters).
type OutstandingPayablesInput struct{}

// GetOutstandingPayables lists all vendor ledger balances under 'Sundry Creditors' sorted descending.
func GetOutstandingPayables(ctx context.Context, req *mcp.CallToolRequest, input OutstandingPayablesInput) (*mcp.CallToolResult, any, error) {
	parentMap, err := fetchGroups(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching group hierarchy: %v", err)), nil, nil
	}

	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching ledger balances: %v", err)), nil, nil
	}

	type creditorItem struct {
		Name    string
		Group   string
		Balance float64
		Type    string // "Dr" or "Cr"
	}

	var creditors []creditorItem
	var debitCreditors []creditorItem
	for _, l := range ledgers {
		if belongsToGroup(l.Parent, "Sundry Creditors", parentMap) {
			if l.ClosingBalance > 0 {
				item := creditorItem{
					Name:    l.Name,
					Group:   l.Parent,
					Balance: l.ClosingBalance,
					Type:    l.ClosingType,
				}
				if l.ClosingType == "Cr" {
					creditors = append(creditors, item)
				} else {
					debitCreditors = append(debitCreditors, item)
				}
			}
		}
	}

	if len(creditors) == 0 && len(debitCreditors) == 0 {
		return textResult("### Outstanding Payables (Sundry Creditors)\n\nNo outstanding payables or vendor debit balances found."), nil, nil
	}

	// Sort descending by balance
	sort.Slice(creditors, func(i, j int) bool {
		return creditors[i].Balance > creditors[j].Balance
	})
	sort.Slice(debitCreditors, func(i, j int) bool {
		return debitCreditors[i].Balance > debitCreditors[j].Balance
	})

	var sb strings.Builder
	sb.WriteString("### Outstanding Payables (Sundry Creditors)\n\n")

	if len(creditors) > 0 {
		sb.WriteString("| Vendor Ledger Name | Group / Parent | Outstanding Balance (Rs) | Type |\n")
		sb.WriteString("| :--- | :--- | :---: | :---: |\n")

		var total float64
		for _, c := range creditors {
			sb.WriteString(fmt.Sprintf("| %s | *%s* | %s | %s |\n", c.Name, c.Group, formatCurrency(c.Balance), c.Type))
			total += c.Balance
		}
		sb.WriteString(fmt.Sprintf("| **TOTAL OUTSTANDING** | | **%s** | **Cr** |\n\n", formatCurrency(total)))
	} else {
		sb.WriteString("No active credit outstanding balances found.\n\n")
	}

	if len(debitCreditors) > 0 {
		sb.WriteString("### Debit Balances in Creditors (Vendor Advances / Prepayments)\n\n")
		sb.WriteString("| Vendor Ledger Name | Group / Parent | Debit Balance (Rs) |\n")
		sb.WriteString("| :--- | :--- | :---: |\n")
		for _, dc := range debitCreditors {
			sb.WriteString(fmt.Sprintf("| %s | *%s* | %s |\n", dc.Name, dc.Group, formatCurrency(dc.Balance)))
		}
	}

	return textResult(sb.String()), nil, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// get_profit_and_loss
// ──────────────────────────────────────────────────────────────────────────────

// ProfitAndLossInput is the input schema for get_profit_and_loss (no parameters).
type ProfitAndLossInput struct{}

// GetProfitAndLoss computes the company's Gross Profit and Net Profit.
func GetProfitAndLoss(ctx context.Context, req *mcp.CallToolRequest, input ProfitAndLossInput) (*mcp.CallToolResult, any, error) {
	parentMap, err := fetchGroups(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching group hierarchy: %v", err)), nil, nil
	}

	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching ledger balances: %v", err)), nil, nil
	}

	var salesLedgers, directIncomeLedgers, purchaseLedgers, directExpenseLedgers []LedgerInfo
	var indirectIncomeLedgers, indirectExpenseLedgers, stockLedgers []LedgerInfo

	for _, l := range ledgers {
		if belongsToGroup(l.Parent, "Sales Accounts", parentMap) {
			salesLedgers = append(salesLedgers, l)
		} else if belongsToGroup(l.Parent, "Direct Incomes", parentMap) {
			directIncomeLedgers = append(directIncomeLedgers, l)
		} else if belongsToGroup(l.Parent, "Purchase Accounts", parentMap) {
			purchaseLedgers = append(purchaseLedgers, l)
		} else if belongsToGroup(l.Parent, "Direct Expenses", parentMap) {
			directExpenseLedgers = append(directExpenseLedgers, l)
		} else if belongsToGroup(l.Parent, "Indirect Incomes", parentMap) {
			indirectIncomeLedgers = append(indirectIncomeLedgers, l)
		} else if belongsToGroup(l.Parent, "Indirect Expenses", parentMap) {
			indirectExpenseLedgers = append(indirectExpenseLedgers, l)
		} else if belongsToGroup(l.Parent, "Stock-in-hand", parentMap) {
			stockLedgers = append(stockLedgers, l)
		}
	}

	// Calculate totals
	var totalSales, totalDirectIncome, totalPurchases, totalDirectExpenses float64
	var totalIndirectIncome, totalIndirectExpenses, totalOpeningStock, totalClosingStock float64

	for _, l := range salesLedgers {
		val := l.ClosingBalance
		if l.ClosingType == "Dr" {
			val = -val
		}
		totalSales += val
	}
	for _, l := range directIncomeLedgers {
		val := l.ClosingBalance
		if l.ClosingType == "Dr" {
			val = -val
		}
		totalDirectIncome += val
	}
	for _, l := range purchaseLedgers {
		val := l.ClosingBalance
		if l.ClosingType == "Cr" {
			val = -val
		}
		totalPurchases += val
	}
	for _, l := range directExpenseLedgers {
		val := l.ClosingBalance
		if l.ClosingType == "Cr" {
			val = -val
		}
		totalDirectExpenses += val
	}
	for _, l := range indirectIncomeLedgers {
		val := l.ClosingBalance
		if l.ClosingType == "Dr" {
			val = -val
		}
		totalIndirectIncome += val
	}
	for _, l := range indirectExpenseLedgers {
		val := l.ClosingBalance
		if l.ClosingType == "Cr" {
			val = -val
		}
		totalIndirectExpenses += val
	}
	for _, l := range stockLedgers {
		valOp := l.OpeningBalance
		if l.OpeningType == "Cr" {
			valOp = -valOp
		}
		totalOpeningStock += valOp

		valCl := l.ClosingBalance
		if l.ClosingType == "Cr" {
			valCl = -valCl
		}
		totalClosingStock += valCl
	}

	grossProfit := (totalSales + totalDirectIncome + totalClosingStock) - (totalOpeningStock + totalPurchases + totalDirectExpenses)
	netProfit := grossProfit + totalIndirectIncome - totalIndirectExpenses

	var sb strings.Builder
	sb.WriteString("## Profit & Loss Statement\n\n")

	// --- TRADING ACCOUNT ---
	sb.WriteString("### 1. Trading Account (Gross Profit Calculation)\n\n")
	sb.WriteString("| Particulars | Debit (Rs) | Credit (Rs) |\n")
	sb.WriteString("| :--- | :---: | :---: |\n")
	sb.WriteString(fmt.Sprintf("| **Revenue from Operations (Sales)** | | %s |\n", formatCurrency(totalSales)))
	if totalDirectIncome > 0 {
		sb.WriteString(fmt.Sprintf("| **Direct Incomes** | | %s |\n", formatCurrency(totalDirectIncome)))
	}
	sb.WriteString(fmt.Sprintf("| **Opening Stock** | %s | |\n", formatCurrency(totalOpeningStock)))
	sb.WriteString(fmt.Sprintf("| **Cost of Purchases** | %s | |\n", formatCurrency(totalPurchases)))
	if totalDirectExpenses > 0 {
		sb.WriteString(fmt.Sprintf("| **Direct Expenses** | %s | |\n", formatCurrency(totalDirectExpenses)))
	}
	sb.WriteString(fmt.Sprintf("| **Closing Stock** | | %s |\n", formatCurrency(totalClosingStock)))

	if grossProfit >= 0 {
		sb.WriteString(fmt.Sprintf("| **Gross Profit c/o** | **%s** | |\n", formatCurrency(grossProfit)))
		sb.WriteString(fmt.Sprintf("| **TOTAL TRADING** | **%s** | **%s** |\n\n",
			formatCurrency(totalOpeningStock+totalPurchases+totalDirectExpenses+grossProfit),
			formatCurrency(totalSales+totalDirectIncome+totalClosingStock)))
	} else {
		sb.WriteString(fmt.Sprintf("| **Gross Loss c/o** | | **%s** |\n", formatCurrency(math.Abs(grossProfit))))
		sb.WriteString(fmt.Sprintf("| **TOTAL TRADING** | **%s** | **%s** |\n\n",
			formatCurrency(totalOpeningStock+totalPurchases+totalDirectExpenses),
			formatCurrency(totalSales+totalDirectIncome+totalClosingStock+math.Abs(grossProfit))))
	}

	// --- INCOME STATEMENT ---
	sb.WriteString("### 2. Profit & Loss Account (Net Profit Calculation)\n\n")
	sb.WriteString("| Particulars | Expenses (Rs) | Incomes (Rs) |\n")
	sb.WriteString("| :--- | :---: | :---: |\n")

	if grossProfit >= 0 {
		sb.WriteString(fmt.Sprintf("| **Gross Profit b/f** | | %s |\n", formatCurrency(grossProfit)))
	} else {
		sb.WriteString(fmt.Sprintf("| **Gross Loss b/f** | %s | |\n", formatCurrency(math.Abs(grossProfit))))
	}

	if totalIndirectIncome > 0 {
		sb.WriteString(fmt.Sprintf("| **Indirect Incomes** | | %s |\n", formatCurrency(totalIndirectIncome)))
	}
	sb.WriteString(fmt.Sprintf("| **Indirect Expenses** | %s | |\n", formatCurrency(totalIndirectExpenses)))

	if netProfit >= 0 {
		sb.WriteString(fmt.Sprintf("| **Net Profit** | **%s** | |\n", formatCurrency(netProfit)))
		totalExpSide := totalIndirectExpenses + netProfit
		if grossProfit < 0 {
			totalExpSide += math.Abs(grossProfit)
		}
		totalIncSide := totalIndirectIncome
		if grossProfit >= 0 {
			totalIncSide += grossProfit
		}
		sb.WriteString(fmt.Sprintf("| **TOTAL P&L** | **%s** | **%s** |\n\n",
			formatCurrency(totalExpSide), formatCurrency(totalIncSide)))
	} else {
		sb.WriteString(fmt.Sprintf("| **Net Loss** | | **%s** |\n", formatCurrency(math.Abs(netProfit))))
		totalExpSide := totalIndirectExpenses
		if grossProfit < 0 {
			totalExpSide += math.Abs(grossProfit)
		}
		totalIncSide := totalIndirectIncome + math.Abs(netProfit)
		if grossProfit >= 0 {
			totalIncSide += grossProfit
		}
		sb.WriteString(fmt.Sprintf("| **TOTAL P&L** | **%s** | **%s** |\n\n",
			formatCurrency(totalExpSide), formatCurrency(totalIncSide)))
	}

	// Detail breakdown
	sb.WriteString("### 3. Detailed Ledger Breakdown\n\n")
	if len(salesLedgers) > 0 {
		sb.WriteString("**Sales Accounts:**\n")
		for _, l := range salesLedgers {
			if l.ClosingBalance > 0 {
				sb.WriteString(fmt.Sprintf("- %s: Rs %s (%s)\n", l.Name, formatCurrency(l.ClosingBalance), l.ClosingType))
			}
		}
		sb.WriteString("\n")
	}
	if len(purchaseLedgers) > 0 {
		sb.WriteString("**Purchase Accounts:**\n")
		for _, l := range purchaseLedgers {
			if l.ClosingBalance > 0 {
				sb.WriteString(fmt.Sprintf("- %s: Rs %s (%s)\n", l.Name, formatCurrency(l.ClosingBalance), l.ClosingType))
			}
		}
		sb.WriteString("\n")
	}
	if len(directExpenseLedgers) > 0 {
		sb.WriteString("**Direct Expenses:**\n")
		sort.Slice(directExpenseLedgers, func(i, j int) bool {
			return directExpenseLedgers[i].ClosingBalance > directExpenseLedgers[j].ClosingBalance
		})
		for _, l := range directExpenseLedgers {
			if l.ClosingBalance > 0 {
				sb.WriteString(fmt.Sprintf("- %s: Rs %s (%s)\n", l.Name, formatCurrency(l.ClosingBalance), l.ClosingType))
			}
		}
		sb.WriteString("\n")
	}
	if len(indirectExpenseLedgers) > 0 {
		sb.WriteString("**Indirect Expenses:**\n")
		sort.Slice(indirectExpenseLedgers, func(i, j int) bool {
			return indirectExpenseLedgers[i].ClosingBalance > indirectExpenseLedgers[j].ClosingBalance
		})
		for _, l := range indirectExpenseLedgers {
			if l.ClosingBalance > 0 {
				sb.WriteString(fmt.Sprintf("- %s: Rs %s (%s)\n", l.Name, formatCurrency(l.ClosingBalance), l.ClosingType))
			}
		}
	}

	return textResult(sb.String()), nil, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// get_balance_sheet
// ──────────────────────────────────────────────────────────────────────────────

// BalanceSheetInput is the input schema for get_balance_sheet (no parameters).
type BalanceSheetInput struct{}

// GetBalanceSheet retrieves a formatted Balance Sheet.
func GetBalanceSheet(ctx context.Context, req *mcp.CallToolRequest, input BalanceSheetInput) (*mcp.CallToolResult, any, error) {
	parentMap, err := fetchGroups(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching group hierarchy: %v", err)), nil, nil
	}

	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching ledger balances: %v", err)), nil, nil
	}

	type balanceItem struct {
		Name    string
		Group   string
		Balance float64
	}

	var capitalItems, loanItems, curLiabItems, suspenseItems, branchLiabItems []balanceItem
	var fixedAssetItems, investmentItems, curAssetItems, miscAssetItems, branchAssetItems []balanceItem
	var plLiability, plAsset float64

	for _, l := range ledgers {
		if l.ClosingBalance == 0 {
			continue
		}

		if strings.EqualFold(l.Name, "Profit & Loss A/c") || strings.EqualFold(l.Name, "Profit and Loss A/c") {
			if l.ClosingType == "Cr" {
				plLiability = l.ClosingBalance
			} else {
				plAsset = l.ClosingBalance
			}
			continue
		}

		if belongsToGroup(l.Parent, "Capital Account", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Dr" {
				val = -val
			}
			capitalItems = append(capitalItems, balanceItem{l.Name, l.Parent, val})
		} else if belongsToGroup(l.Parent, "Loans (Liability)", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Dr" {
				val = -val
			}
			loanItems = append(loanItems, balanceItem{l.Name, l.Parent, val})
		} else if belongsToGroup(l.Parent, "Current Liabilities", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Dr" {
				val = -val
			}
			curLiabItems = append(curLiabItems, balanceItem{l.Name, l.Parent, val})
		} else if belongsToGroup(l.Parent, "Suspense A/c", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Dr" {
				val = -val
			}
			suspenseItems = append(suspenseItems, balanceItem{l.Name, l.Parent, val})
		} else if belongsToGroup(l.Parent, "Branch / Divisions", parentMap) {
			if l.ClosingType == "Cr" {
				branchLiabItems = append(branchLiabItems, balanceItem{l.Name, l.Parent, l.ClosingBalance})
			} else {
				branchAssetItems = append(branchAssetItems, balanceItem{l.Name, l.Parent, l.ClosingBalance})
			}
		} else if belongsToGroup(l.Parent, "Fixed Assets", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Cr" {
				val = -val
			}
			fixedAssetItems = append(fixedAssetItems, balanceItem{l.Name, l.Parent, val})
		} else if belongsToGroup(l.Parent, "Investments", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Cr" {
				val = -val
			}
			investmentItems = append(investmentItems, balanceItem{l.Name, l.Parent, val})
		} else if belongsToGroup(l.Parent, "Current Assets", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Cr" {
				val = -val
			}
			curAssetItems = append(curAssetItems, balanceItem{l.Name, l.Parent, val})
		} else if belongsToGroup(l.Parent, "Miscellaneous Expenses (Asset)", parentMap) {
			val := l.ClosingBalance
			if l.ClosingType == "Cr" {
				val = -val
			}
			miscAssetItems = append(miscAssetItems, balanceItem{l.Name, l.Parent, val})
		}
	}

	var sumCapital, sumLoans, sumCurLiab, sumSuspense, sumBranchLiab float64
	var sumFixedAssets, sumInvestments, sumCurAssets, sumMiscAssets, sumBranchAssets float64

	for _, item := range capitalItems {
		sumCapital += item.Balance
	}
	for _, item := range loanItems {
		sumLoans += item.Balance
	}
	for _, item := range curLiabItems {
		sumCurLiab += item.Balance
	}
	for _, item := range suspenseItems {
		sumSuspense += item.Balance
	}
	for _, item := range branchLiabItems {
		sumBranchLiab += item.Balance
	}

	for _, item := range fixedAssetItems {
		sumFixedAssets += item.Balance
	}
	for _, item := range investmentItems {
		sumInvestments += item.Balance
	}
	for _, item := range curAssetItems {
		sumCurAssets += item.Balance
	}
	for _, item := range miscAssetItems {
		sumMiscAssets += item.Balance
	}
	for _, item := range branchAssetItems {
		sumBranchAssets += item.Balance
	}

	totalLiabilities := sumCapital + sumLoans + sumCurLiab + sumSuspense + sumBranchLiab + plLiability
	totalAssets := sumFixedAssets + sumInvestments + sumCurAssets + sumMiscAssets + sumBranchAssets + plAsset

	var sb strings.Builder
	sb.WriteString("## Balance Sheet\n\n")

	sb.WriteString("### 1. Summary Statement\n\n")
	sb.WriteString("| Capital & Liabilities | Amount (Rs) | Assets | Amount (Rs) |\n")
	sb.WriteString("| :--- | :---: | :--- | :---: |\n")

	sb.WriteString(fmt.Sprintf("| **Capital Account** | %s | **Fixed Assets** | %s |\n",
		formatCurrency(sumCapital), formatCurrency(sumFixedAssets)))

	sb.WriteString(fmt.Sprintf("| **Loans (Liability)** | %s | **Investments** | %s |\n",
		formatCurrency(sumLoans), formatCurrency(sumInvestments)))

	sb.WriteString(fmt.Sprintf("| **Current Liabilities** | %s | **Current Assets** | %s |\n",
		formatCurrency(sumCurLiab), formatCurrency(sumCurAssets)))

	sb.WriteString(fmt.Sprintf("| **Suspense A/c** | %s | **Misc. Expenses (Asset)** | %s |\n",
		formatCurrency(sumSuspense), formatCurrency(sumMiscAssets)))

	sb.WriteString(fmt.Sprintf("| **Branch / Divisions (Liab)** | %s | **Branch / Divisions (Asset)** | %s |\n",
		formatCurrency(sumBranchLiab), formatCurrency(sumBranchAssets)))

	plLiabStr := "-"
	if plLiability > 0 {
		plLiabStr = formatCurrency(plLiability)
	}
	plAssetStr := "-"
	if plAsset > 0 {
		plAssetStr = formatCurrency(plAsset)
	}
	sb.WriteString(fmt.Sprintf("| **Profit & Loss A/c (Retained)** | %s | **Profit & Loss A/c (Loss)** | %s |\n",
		plLiabStr, plAssetStr))

	sb.WriteString(fmt.Sprintf("| **TOTAL LIABILITIES** | **%s** | **TOTAL ASSETS** | **%s** |\n\n",
		formatCurrency(totalLiabilities), formatCurrency(totalAssets)))

	diff := math.Abs(totalLiabilities - totalAssets)
	if diff > 0.01 {
		sb.WriteString(fmt.Sprintf("> [!WARNING]\n> **Difference in Opening Balances / Discrepancy**: Rs %s\n\n", formatCurrency(diff)))
	} else {
		sb.WriteString("> [!NOTE]\n> **Balance Sheet status**: Balanced successfully.\n\n")
	}

	sb.WriteString("> [!NOTE]\n> **Zero-Balance Ledgers**: Zero-balance ledgers are excluded from the Balance Sheet.\n\n")
	sb.WriteString("> [!NOTE]\n> **Fixed Assets**: Rs 0.00 (Capital equipment/assets like Laptops, Printers, etc., are currently tracked under stock inventory rather than Fixed Asset ledgers in Tally).\n\n")

	sb.WriteString("### 2. Detailed Assets Breakdown\n\n")
	if len(fixedAssetItems) > 0 {
		sb.WriteString("**Fixed Assets:**\n")
		for _, item := range fixedAssetItems {
			sb.WriteString(fmt.Sprintf("- %s: Rs %s\n", item.Name, formatCurrency(item.Balance)))
		}
		sb.WriteString("\n")
	}
	if len(curAssetItems) > 0 {
		sb.WriteString("**Current Assets:**\n")
		for _, item := range curAssetItems {
			sb.WriteString(fmt.Sprintf("- %s: Rs %s (*%s*)\n", item.Name, formatCurrency(item.Balance), item.Group))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### 3. Detailed Liabilities Breakdown\n\n")
	if len(capitalItems) > 0 {
		sb.WriteString("**Capital Accounts:**\n")
		for _, item := range capitalItems {
			sb.WriteString(fmt.Sprintf("- %s: Rs %s\n", item.Name, formatCurrency(item.Balance)))
		}
		sb.WriteString("\n")
	}
	if len(curLiabItems) > 0 {
		sb.WriteString("**Current Liabilities:**\n")
		for _, item := range curLiabItems {
			sb.WriteString(fmt.Sprintf("- %s: Rs %s (*%s*)\n", item.Name, formatCurrency(item.Balance), item.Group))
		}
	}

	return textResult(sb.String()), nil, nil
}
