package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/omesh7/tallymcp/internal/tally"
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
		Name:        "list_stock_items",
		Description: "Lists all inventory stock items populated in Tally Prime, with an optional prefix filter.",
	}, ListStockItems)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_ledgers",
		Description: "Lists all ledger accounts in Tally Prime, with an optional group/parent filter (e.g. Sundry Debtors).",
	}, ListLedgers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_day_book",
		Description: "Retrieves a listing of all vouchers (transactions) recorded in Tally Prime for a date or date range.",
	}, GetDayBook)

	// ── Write Tools ─────────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_quick_journal_voucher",
		Description: "Safely records a balanced Journal voucher between two ledgers on an Educational-safe date (1st or 2nd of April 2026).",
	}, AddQuickJournalVoucher)

	// ── Analytics Tools ─────────────────────────────────────────────────────

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_sales_analytics",
		Description: "Performs month-on-month sales calculations and returns growth percentages for the active company.",
	}, GetSalesAnalytics)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_expense_analytics",
		Description: "Fetches multi-period expenses and sales to calculate net profit margins and profitability trends.",
	}, GetExpenseAnalytics)
}
