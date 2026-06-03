# TallyMCP — Bridge Claude and other AI models to Tally Prime

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-blue?style=for-the-badge&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![Protocol](https://img.shields.io/badge/MCP-Compatible-orange?style=for-the-badge)](https://modelcontextprotocol.io)

**TallyMCP** is an open-source **Model Context Protocol (MCP) server** that bridges AI assistants (such as Claude Desktop, Cursor, VS Code, and Antigravity IDE) directly to **Tally Prime**, India's leading accounting and ERP software.

Using TallyMCP, you can interact with your local Tally Prime company data in natural language: query balances, get monthly analytics, list active companies, view stock levels, and record balanced transactions.

---

> [!WARNING]
> ### ⚠️ CRITICAL DISCLAIMER: AI CAN MAKE MISTAKES
> * **Just a Bridge**: TallyMCP acts purely as a communication bridge between your AI client and your local Tally Prime instance. It does **not** validate the accounting correctness of the transactions or reports.
> * **Double-Check Entries**: AI models can hallucinate or interpret natural language queries incorrectly. **Always double-check and verify** any entries created (such as Journal Vouchers) and numbers reported before acting on them for regulatory or business decisions.
> * **Model Choice Matters**: The accuracy of queries and transactions depends heavily on the capability of the underlying LLM. **TallyMCP has been heavily tested and optimized using Claude 4.5 Sonnet / Claude 4 Opus**. Using less capable models may result in incorrect parameter mappings.

---

## 🏗️ Architecture

TallyMCP is designed with a **local-first** security model. Your accounting data never leaves your machine. The Go binary communicates with Tally Prime's local XML server running on `localhost:9000`.

```
┌──────────────────┐     stdio (JSON-RPC 2.0)     ┌──────────────────────┐
│  AI Client       │ ◄───────────────────────────► │  tallymcp.exe        │
│  (Claude Desktop)│    stdin: requests           │  (Go Single Binary)  │
│                  │    stdout: responses          │                      │
└──────────────────┘    stderr: log messages      └──────────┬───────────┘
                                                             │
                                                  HTTP POST (UTF-16LE XML)
                                                  port 9000, Connection: close
                                                  Mutex-serialized requests
                                                             │
                                                             ▼
                                                 ┌───────────────────────┐
                                                 │    Tally Prime        │
                                                 │    (Windows Desktop)  │
                                                 │    XML Server Mode    │
                                                 └───────────────────────┘
```

---

## ⚡ 30-Second Quickstart

### Prerequisites
1. **Tally Prime** running on Windows.
2. XML Server enabled on port `9000` (check: *F1: Help > Settings > Connectivity > Enable HTTP/XML services: Yes, Port: 9000*).
3. A company loaded (active) in Tally Prime.

### Step 1: Build the Executable
```powershell
cd tallymcp-go
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```
This produces a single, dependency-free binary `tallymcp.exe` under `tallymcp-go/`.

### Step 2: Register in Claude Desktop
Open your Claude Desktop configuration file (typically `%APPDATA%\Claude\claude_desktop_config.json`) and add the server:

```json
{
  "mcpServers": {
    "tally-mcp-server": {
      "command": "C:/path/to/tallymcp-go/tallymcp.exe"
    }
  }
}
```
*(Replace the path above with the absolute path to your compiled `tallymcp.exe`)*.

Restart Claude Desktop, and you will see the Tally tools icon available in your chats.

---

## 🛠️ Tool Reference

TallyMCP exposes **13 core tools** for AI model consumption:

| # | Tool Name | Type | Description | Key Parameters |
|---|-----------|------|-------------|----------------|
| 1 | `list_companies` | Read | Lists all loaded Tally companies and highlights the active one. | None |
| 2 | `get_trial_balance` | Read | Fetches a full trial balance with debit/credit columns. | None |
| 3 | `get_ledger_closing_balance` | Read | Retrieves closing balance of a specific ledger with optional date filter. | `ledger_name`, `date` (optional) |
| 4 | `list_stock_items` | Read | Lists inventory stock items with quantities, rates, and values. Supports prefix filters. | `prefix` (optional) |
| 5 | `list_ledgers` | Read | Lists all ledger accounts, with optional parent group filtering (e.g. Bank Accounts). | `group` (optional) |
| 6 | `get_day_book` | Read | Retrieves a list of all recorded transactions (vouchers) in Tally with their amounts and types. | `date` (optional) |
| 7 | `add_quick_journal_voucher` | Write | Records a balanced Journal voucher between two ledgers. (EDU Mode safe dates) | `debit_ledger`, `credit_ledger`, `amount`, `narration`, `day` |
| 8 | `get_sales_analytics` | Read | Generates month-on-month sales aggregates and growth percentages for a sales ledger. | `sales_ledger` (optional) |
| 9 | `get_expense_analytics` | Read | Compiles multi-period profitability and net margin trends using custom expenses/sales. | `sales_ledger` (optional), `expense_ledgers` (optional) |
| 10 | `get_outstanding_receivables` | Read | Lists all outstanding customer receivables (Sundry Debtors) sorted descending by balance. | None |
| 11 | `get_outstanding_payables` | Read | Lists all outstanding vendor payables (Sundry Creditors) sorted descending by balance. | None |
| 12 | `get_profit_and_loss` | Read | Calculates Gross Profit and Net Profit dynamically based on revenue, expense, and stock. | None |
| 13 | `get_balance_sheet` | Read | Fetches a structured Balance Sheet showing Capital & Liabilities vs. Assets. | None |

---

## 🛡️ Security Model
* **Local-Only Communication**: The server only listens and talks on localhost. There is no remote dashboard, database, or telemetry.
* **Input Validation**: All parameters are strongly typed and validated before sending queries. Ledger names, narration strings, and filters are fully XML-escaped (`xmlEscape`) to prevent XML injection.
* **Mutex Serializer**: Tally's HTTP port 9000 is single-threaded. TallyMCP uses a sync Mutex to serialize requests, protecting your Tally instance from concurrent request freezes.
* **Disable Keep-Alives**: Explicitly closes TCP sockets after each request to prevent port exhaustion on Windows machines.

---

## 🗺️ Roadmap
* **v0.1.0** (Current): Core Go rewrite, safety validations, unit testing.
* **v0.2.0**: Bulk voucher import from Excel/CSV, support for remote cloud Tally via SSE.
* **v0.3.0**: Automated GST reconciliation and outstanding payment aging analysis.

---

## 🤝 Contributing
Contributions are welcome! Please check out [CONTRIBUTING.md](CONTRIBUTING.md) to understand Go styles, tool guidelines, and our pull request process.

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
