package tools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/omesh7/tally-mcp/internal/tally"
)

func TestLiveReports(t *testing.T) {
	// Connect to local Tally Prime
	client := tally.NewClient("localhost", 9000, 5*time.Second)
	tallyClient = client

	ctx := context.Background()
	// Quick ping
	_, err := client.PostXML(ctx, "<ENVELOPE></ENVELOPE>")
	if err != nil {
		t.Skip("Tally Prime is not running on localhost:9000, skipping live integration tests")
		return
	}

	t.Run("GetOutstandingReceivables", func(t *testing.T) {
		res, _, err := GetOutstandingReceivables(ctx, nil, OutstandingReceivablesInput{})
		if err != nil {
			t.Fatalf("Error in GetOutstandingReceivables: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if text == "" {
			t.Errorf("Empty response from GetOutstandingReceivables")
		}
		fmt.Printf("--- Outstanding Receivables ---\n%s\n", text)
	})

	t.Run("GetOutstandingPayables", func(t *testing.T) {
		res, _, err := GetOutstandingPayables(ctx, nil, OutstandingPayablesInput{})
		if err != nil {
			t.Fatalf("Error in GetOutstandingPayables: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if text == "" {
			t.Errorf("Empty response from GetOutstandingPayables")
		}
		fmt.Printf("--- Outstanding Payables ---\n%s\n", text)
	})

	t.Run("GetProfitAndLoss", func(t *testing.T) {
		res, _, err := GetProfitAndLoss(ctx, nil, ProfitAndLossInput{})
		if err != nil {
			t.Fatalf("Error in GetProfitAndLoss: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if text == "" {
			t.Errorf("Empty response from GetProfitAndLoss")
		}
		fmt.Printf("--- Profit & Loss ---\n%s\n", text)
	})

	t.Run("GetBalanceSheet", func(t *testing.T) {
		res, _, err := GetBalanceSheet(ctx, nil, BalanceSheetInput{})
		if err != nil {
			t.Fatalf("Error in GetBalanceSheet: %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if text == "" {
			t.Errorf("Empty response from GetBalanceSheet")
		}
		fmt.Printf("--- Balance Sheet ---\n%s\n", text)
	})
}
