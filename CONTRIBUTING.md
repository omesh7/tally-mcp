# Contributing to TallyMCP

Thank you for your interest in contributing to TallyMCP! We welcome issues, suggestions, and pull requests to help make this bridge powerful and reliable for everyone.

---

## 🏗️ Codebase Structure

TallyMCP is written in **Go** and utilizes the official Model Context Protocol Go SDK.

```
post-002-tally/
├── tallymcp-go/
│   ├── main.go                      # Entry point: registers tools and runs the STDIO transport.
│   ├── internal/
│   │   ├── config/                  # Configuration loader (env vars and defaults).
│   │   ├── tally/                   # Tally HTTP client, encoding, and XML response parser.
│   │   ├── xml/                     # Composable, fluent XML and TDL builder.
│   │   └── tools/                   # MCP tool definitions and handlers.
│   └── scripts/
│       └── build.ps1                # Windows PowerShell ASCII build script.
```

---

## 🛠️ How to Add a New Tool

Adding a tool to TallyMCP is designed to be simple and modular.

### Step 1: Create a Tool Handler
Create or open a file under `internal/tools/` (e.g., `internal/tools/outstanding.go`). Define your input schema and the handler function:

```go
package tools

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tallymcp/internal/xml"
)

// OutstandingInput represents the input schema for the tool.
type OutstandingInput struct {
	LedgerName string `json:"ledger_name" jsonschema:"The exact ledger to search for"`
}

// GetOutstandingBills retrieves outstanding records from Tally.
func GetOutstandingBills(ctx context.Context, req *mcp.CallToolRequest, input OutstandingInput) (*mcp.CallToolResult, any, error) {
	if input.LedgerName == "" {
		return textResult("❌ **Error:** ledger_name is a required parameter."), nil, nil
	}

	// Build compose XML/TDL
	tdl := tallyxml.NewTDL().
		// Build TDL report structure...
	
	xmlStr := buildStandardExportQuery(tdl, nil)
	
	// Post and parse
	root, err := postAndParse(ctx, xmlStr)
	if err != nil {
		return textResult(fmt.Sprintf("Error fetching outstanding: %v", err)), nil, nil
	}

	// Format output and return
	resultText := fmt.Sprintf("### Outstanding Bills for %s\n...", input.LedgerName)
	return textResult(resultText), nil, nil
}
```

### Step 2: Register the Tool
Open `internal/tools/register.go` and append your tool to the `RegisterAll` method:

```go
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_outstanding_bills",
		Description: "Retrieves outstanding bills for a specific vendor or customer ledger.",
	}, GetOutstandingBills)
```

The MCP SDK will automatically parse the `OutstandingInput` Go struct tags and generate the corresponding JSON-Schema parameters for the AI model.

---

## 📏 Coding Standards & Security Rules

1. **Stdout is Sacred**: Never print anything to `stdout` (`fmt.Println`, `print`, etc.) during runtime unless it is handled by the MCP SDK. Stray prints will corrupt the JSON-RPC pipe and crash the AI client. Send all logs to `stderr` or use `log.Printf`.
2. **XML Escaping**: Always escape user inputs placed into XML strings. Use `tallyxml.xmlEscape()` or filters like `tallyxml.EqualFilter()`.
3. **Single-threaded Mutex**: Never bypass `tallyClient.PostXML()` since it manages a connection mutex. Tally Prime port 9000 is single-threaded; concurrent requests will cause dropped sockets.
4. **Go Formatting**: Always run `gofmt -s -w .` before committing changes.

---

## 🧪 Testing

We use standard Go unit tests. Write tests for your logic under the corresponding package (e.g., `parser_test.go` or `builder_test.go`).

To run the unit tests:
```powershell
cd tallymcp-go
go test -v ./...
```

To run a lint check on syntax:
```powershell
go vet ./...
```
