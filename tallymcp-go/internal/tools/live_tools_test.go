package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/omesh7/tally-mcp/internal/tally"
	tallyxml "github.com/omesh7/tally-mcp/internal/xml"
)

func TestLiveReadTools(t *testing.T) {
	client := tally.NewClient("localhost", 9000, 5*time.Second)
	tallyClient = client

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Skip("Tally Prime is not running on localhost:9000, skipping live read tool tests")
	}

	t.Run("DayBookDateFilter", func(t *testing.T) {
		res, _, err := GetDayBook(ctx, nil, DayBookInput{Date: "1900-01-01"})
		if err != nil {
			t.Fatalf("GetDayBook returned error: %v", err)
		}
		text := resultText(t, res)
		if !strings.Contains(text, "No vouchers found") {
			t.Fatalf("date filter did not restrict Day Book to empty 1900-01-01 result:\n%s", text)
		}
	})

	t.Run("SalesAnalyticsDynamic", func(t *testing.T) {
		res, _, err := GetSalesAnalytics(ctx, nil, SalesAnalyticsInput{})
		if err != nil {
			t.Fatalf("GetSalesAnalytics returned error: %v", err)
		}
		text := resultText(t, res)
		if strings.Contains(text, "(Sales)") {
			t.Fatalf("sales analytics fell back to hardcoded Sales ledger:\n%s", text)
		}
	})

	t.Run("ExpenseAnalyticsDynamic", func(t *testing.T) {
		res, _, err := GetExpenseAnalytics(ctx, nil, ExpenseAnalyticsInput{})
		if err != nil {
			t.Fatalf("GetExpenseAnalytics returned error: %v", err)
		}
		text := resultText(t, res)
		if !strings.Contains(text, "Expense ledgers:") {
			t.Fatalf("expense analytics did not report resolved expense ledgers:\n%s", text)
		}
	})

	ledger := firstLiveLedger(t, ctx)
	t.Run("LedgerStatement", func(t *testing.T) {
		res, _, err := GetLedgerStatement(ctx, nil, LedgerStatementInput{LedgerName: ledger})
		if err != nil {
			t.Fatalf("GetLedgerStatement returned error: %v", err)
		}
		if text := resultText(t, res); text == "" {
			t.Fatal("empty ledger statement response")
		}
	})

	stockItem := firstLiveStockItem(t, ctx)
	if stockItem == "" {
		t.Skip("No stock items available for stock movement live test")
	}
	t.Run("StockMovement", func(t *testing.T) {
		res, _, err := GetStockMovement(ctx, nil, StockMovementInput{StockItemName: stockItem})
		if err != nil {
			t.Fatalf("GetStockMovement returned error: %v", err)
		}
		if text := resultText(t, res); text == "" {
			t.Fatal("empty stock movement response")
		}
	})
}

func firstLiveLedger(t *testing.T, ctx context.Context) string {
	t.Helper()
	ledgers, err := fetchLedgers(ctx)
	if err != nil {
		t.Fatalf("fetchLedgers failed: %v", err)
	}
	for _, l := range ledgers {
		if l.Name != "" && l.ClosingBalance != 0 {
			return l.Name
		}
	}
	if len(ledgers) > 0 {
		return ledgers[0].Name
	}
	t.Fatal("no ledgers found")
	return ""
}

func firstLiveStockItem(t *testing.T, ctx context.Context) string {
	t.Helper()
	tdl := tallyxml.NewTDL().
		Report("R", "F").
		Form("F", "DATA", "P").
		Part("P", "L", "Col").
		Line("L", "ROW", "FN").
		Field("FN", "Name", tallyxml.FormulaName).
		Collection("Col", tallyxml.CollectionStockItem)

	root, err := postAndParse(ctx, buildStandardExportQuery(tdl, nil))
	if err != nil {
		t.Fatalf("stock item query failed: %v", err)
	}
	for _, row := range root.FindAll("ROW") {
		if name := strings.TrimSpace(row.FindText("Name")); name != "" {
			return name
		}
	}
	return ""
}
