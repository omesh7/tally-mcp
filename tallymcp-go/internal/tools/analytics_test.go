package tools

import "testing"

func TestMonthKeyFromDateSupportsGeneralTallyDates(t *testing.T) {
	tests := map[string]string{
		"20260401":    "2026-04",
		"2026-12-31":  "2026-12",
		"1-Jan-26":    "2026-01",
		"03/06/2026":  "2026-06",
		"15-Aug-2026": "2026-08",
	}
	for input, want := range tests {
		if got := monthKeyFromDate(input); got != want {
			t.Fatalf("monthKeyFromDate(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLedgerFilterFormulaDoesNotInventBusinessLedgers(t *testing.T) {
	if got := ledgerFilterFormula(nil); got != "" {
		t.Fatalf("ledgerFilterFormula(nil) = %q, want empty", got)
	}
	if got := ledgerFilterFormula([]string{"", "  "}); got != "" {
		t.Fatalf("ledgerFilterFormula(blank) = %q, want empty", got)
	}
}
