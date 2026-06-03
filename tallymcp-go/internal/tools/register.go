package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/omesh7/tally-mcp/internal/tally"
)

// RegisterAll registers all TallyMCP tools with the MCP server.
//
// This is the single wiring point — main.go calls this once to connect
// all tool handlers to the MCP server and share the Tally HTTP client.
func RegisterAll(server *mcp.Server, client *tally.Client) {
	// Store client for use by all handlers
	tallyClient = client

	// ── Read Tools ──────────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_companies",
		Description: "Lists all loaded companies in Tally Prime, indicating which company is currently active.",
	}, ListCompanies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_trial_balance",
		Description: "Fetches a summarized trial balance showing debit and credit balances for all active ledger accounts in the company.",
	}, GetTrialBalance)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ledger_closing_balance",
		Description: "Retrieves the current closing balance of a specific ledger account (e.g., Bank Account, Capital Account, or Customer/Vendor ledger). Optionally accepts a date to get balance as of that date.",
	}, GetLedgerClosingBalance)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ledger_statement",
		Description: "Lists voucher transactions for a ledger over an optional date range.",
	}, GetLedgerStatement)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_stock_items",
		Description: "Lists all inventory stock items populated in Tally Prime, with optional prefix and category/group filters (case-insensitive).",
	}, ListStockItems)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_stock_movement",
		Description: "Summarizes inward and outward movement for a stock item over an optional date range.",
	}, GetStockMovement)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_ledgers",
		Description: "Lists all ledger accounts in Tally Prime, with an optional group/parent filter (e.g. Sundry Debtors).",
	}, ListLedgers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_day_book",
		Description: "Retrieves a listing of all vouchers (transactions) recorded in Tally Prime for a date or date range.",
	}, GetDayBook)

	// ── Financial Reporting Tools ───────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_outstanding_receivables",
		Description: "Lists all outstanding customer receivables (balances under Sundry Debtors) sorted descending by balance, helping you track outstanding invoice amounts.",
	}, GetOutstandingReceivables)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_outstanding_payables",
		Description: "Lists all outstanding vendor payables (balances under Sundry Creditors) sorted descending by balance, helping you track liabilities and due payments.",
	}, GetOutstandingPayables)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_profit_and_loss",
		Description: "Calculates the company's Gross Profit and Net Profit dynamically based on revenue, purchases, direct/indirect expenses, and opening/closing stock.",
	}, GetProfitAndLoss)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_balance_sheet",
		Description: "Fetches a structured Balance Sheet showing Capital & Liabilities vs. Assets, verifying if accounts balance successfully.",
	}, GetBalanceSheet)

	// ── Write Tools (DISABLED) ──────────────────────────────────────────────
	// Write operations are currently disabled for safety.
	/*
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_quick_journal_voucher",
		Description: "Safely records a balanced Journal voucher between two ledgers on an Educational-safe date (1st or 2nd of April 2026).",
	}, AddQuickJournalVoucher)
	*/

	// ── Analytics Tools ─────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_sales_analytics",
		Description: "Performs month-on-month sales calculations and returns growth percentages for the active company. If sales_ledger is omitted, all ledgers under the Sales Accounts group are combined and analyzed.",
	}, GetSalesAnalytics)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_expense_analytics",
		Description: "Fetches multi-period expenses and sales to calculate net profit margins and profitability trends.",
	}, GetExpenseAnalytics)
}
