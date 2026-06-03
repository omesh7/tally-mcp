package tools

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

// StockItemsInput is the input schema for list_stock_items.
type StockItemsInput struct {
	Prefix string `json:"prefix,omitempty" jsonschema:"Optional prefix to filter stock items by name (e.g. 'SC ')"`
}

// ListStockItems lists all stock items, with an optional name prefix filter.
func ListStockItems(ctx context.Context, req *mcp.CallToolRequest, input StockItemsInput) (*mcp.CallToolResult, any, error) {
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN", "FP", "FU", "F_QTY", "F_VAL").
		Field("FN", "Name", tallyxml.FormulaName).
		Field("FP", "Parent", tallyxml.FormulaParent).
		Field("FU", "Unit", tallyxml.FormulaBaseUnits).
		Field("F_QTY", "Qty", tallyxml.FormulaOpeningQty).
		Field("F_VAL", "Val", tallyxml.FormulaOpeningValue).
		Collection("Col", tallyxml.CollectionStockItem)

	xml := buildStandardExportQuery(tdl, nil)

	root, err := postAndParse(ctx, xml)
	if err != nil {
		return textResult(fmt.Sprintf("Error listing stock items: %v", err)), nil, nil
	}

	type stockItem struct {
		name, parent, unit string
		qty, rate, val     float64
	}

	var items []stockItem

	for _, row := range root.FindAll("ROW") {
		name := row.FindText("Name")

		// Filter by prefix if specified
		if input.Prefix != "" && !strings.HasPrefix(name, input.Prefix) {
			continue
		}

		parent := row.FindText("Parent")
		unit := row.FindText("Unit")
		qty := parseFloat(row.FindText("Qty"))
		val := math.Abs(parseFloat(row.FindText("Val")))
		rate := 0.0
		if qty > 0 {
			rate = val / qty
		}

		items = append(items, stockItem{name, parent, unit, qty, rate, val})
	}

	if len(items) == 0 {
		return textResult("No Stock Items found in Tally matching the criteria."), nil, nil
	}

	var sb strings.Builder
	sb.WriteString("### Stock Items & Inventory:\n\n")
	sb.WriteString("| Item Name | Group Category | Unit Type | Opening Qty | Avg Cost (Rs) | Total Value (Rs) |\n")
	sb.WriteString("| :--- | :--- | :--- | :---: | :---: | :---: |\n")

	for _, item := range items {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f | %s | %s |\n",
			item.name, item.parent, item.unit, item.qty, formatCurrency(item.rate), formatCurrency(item.val)))
	}

	return textResult(sb.String()), nil, nil
}
