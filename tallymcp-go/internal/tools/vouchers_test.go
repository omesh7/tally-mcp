package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("empty tool result")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", res.Content[0])
	}
	return text.Text
}

func TestAddQuickJournalVoucherRejectsSameLedgerBeforePosting(t *testing.T) {
	res, _, err := AddQuickJournalVoucher(context.Background(), nil, JournalVoucherInput{
		DebitLedger:  "Bank Account",
		CreditLedger: " bank   account ",
		Amount:       100,
	})
	if err != nil {
		t.Fatalf("AddQuickJournalVoucher returned error: %v", err)
	}
	text := resultText(t, res)
	if !strings.Contains(text, "must be different") {
		t.Fatalf("expected same-ledger rejection, got:\n%s", text)
	}
}

func TestAddQuickJournalVoucherRejectsUnsafeAmountBeforePosting(t *testing.T) {
	res, _, err := AddQuickJournalVoucher(context.Background(), nil, JournalVoucherInput{
		DebitLedger:  "Expense",
		CreditLedger: "Bank",
		Amount:       maxJournalVoucherAmount + 1,
	})
	if err != nil {
		t.Fatalf("AddQuickJournalVoucher returned error: %v", err)
	}
	text := resultText(t, res)
	if !strings.Contains(text, "safety limit") {
		t.Fatalf("expected amount ceiling rejection, got:\n%s", text)
	}
}

func TestJournalVoucherDateRejectsInvalidDay(t *testing.T) {
	_, _, err := journalVoucherDate(JournalVoucherInput{Day: 3})
	if err == nil {
		t.Fatal("expected invalid day error")
	}
	if !strings.Contains(err.Error(), "day must be 1 or 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJournalVoucherDateAcceptsExplicitDate(t *testing.T) {
	date, display, err := journalVoucherDate(JournalVoucherInput{Date: "2026-06-03", Day: 3})
	if err != nil {
		t.Fatalf("unexpected date error: %v", err)
	}
	if date != "20260603" || display != "2026-06-03" {
		t.Fatalf("unexpected date/display: %q %q", date, display)
	}
}
