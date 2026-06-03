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
	Prefix   string `json:"prefix,omitempty" jsonschema:"Optional prefix to filter stock items by name (case-insensitive)"`
	Category string `json:"category,omitempty" jsonschema:"Optional category/group name to filter stock items (case-insensitive)"`
}

// ListStockItems lists all stock items, with optional prefix and category filters.
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
		parent := row.FindText("Parent")

		// Filter by prefix if specified (case-insensitive)
		if input.Prefix != "" && !strings.HasPrefix(strings.ToUpper(name), strings.ToUpper(input.Prefix)) {
			continue
		}

		// Filter by category if specified (case-insensitive exact match)
		if input.Category != "" && !strings.EqualFold(parent, input.Category) {
			continue
		}

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
