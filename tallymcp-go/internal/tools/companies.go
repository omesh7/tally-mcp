package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

// ListCompaniesInput is the input schema for list_companies (no parameters needed).
type ListCompaniesInput struct{}

// ListCompanies fetches all loaded companies in Tally Prime and indicates which is active.
func ListCompanies(ctx context.Context, req *mcp.CallToolRequest, input ListCompaniesInput) (*mcp.CallToolResult, any, error) {
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FA").
		Field("FN", "Name", tallyxml.FormulaName).
		Field("FA", "Active", `if $$IsEqual:$Name:##SVCurrentCompany then 1 else 0`).
		Collection("Col", tallyxml.CollectionCompany)

	xml := buildStandardExportQuery(tdl, nil)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error connecting to Tally Prime: %v\nIs Tally Prime running on port 9000?", err)), nil, nil
	}

	rows := root.FindAll("ROW")
	if len(rows) == 0 {
		return textResult("No companies loaded in Tally Prime. Please load a company."), nil, nil
	}

	var sb strings.Builder
	sb.WriteString("### Loaded Companies in Tally Prime:\n")
	for _, row := range rows {
		name := row.FindText("Name")
		active := row.FindText("Active")
		if active == "1" {
			sb.WriteString(fmt.Sprintf("- **%s** (ACTIVE COMPANY)\n", name))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s**\n", name))
		}
	}

	return textResult(sb.String()), nil, nil
}

// textResult is a helper to create a simple text MCP tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}
