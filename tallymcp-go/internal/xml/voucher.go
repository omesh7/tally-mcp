package tallyxml

import (
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Voucher Builder — Builds TALLYMESSAGE/VOUCHER XML for imports
// ──────────────────────────────────────────────────────────────────────────────

// LedgerEntry represents a single ledger entry within a voucher.
type LedgerEntry struct {
	LedgerName      string
	IsDeemedPositive bool   // Yes for debit in Journal, No for credit
	Amount          float64 // Negative for debit (Tally convention)
}

// VoucherBuilder constructs a TALLYMESSAGE containing a VOUCHER for import.
type VoucherBuilder struct {
	voucherType string
	date        string // YYYYMMDD format
	number      string
	narration   string
	partyLedger string
	entries     []LedgerEntry
}

// NewJournalVoucher creates a builder for a Journal voucher.
func NewJournalVoucher(date string) *VoucherBuilder {
	return &VoucherBuilder{
		voucherType: "Journal",
		date:        date,
	}
}

// NewSalesVoucher creates a builder for a Sales voucher.
func NewSalesVoucher(date, party string) *VoucherBuilder {
	return &VoucherBuilder{
		voucherType: "Sales",
		date:        date,
		partyLedger: party,
	}
}

// NewPurchaseVoucher creates a builder for a Purchase voucher.
func NewPurchaseVoucher(date, party string) *VoucherBuilder {
	return &VoucherBuilder{
		voucherType: "Purchase",
		date:        date,
		partyLedger: party,
	}
}

// NewPaymentVoucher creates a builder for a Payment voucher.
func NewPaymentVoucher(date string) *VoucherBuilder {
	return &VoucherBuilder{
		voucherType: "Payment",
		date:        date,
	}
}

// NewReceiptVoucher creates a builder for a Receipt voucher.
func NewReceiptVoucher(date, party string) *VoucherBuilder {
	return &VoucherBuilder{
		voucherType: "Receipt",
		date:        date,
		partyLedger: party,
	}
}

// Number sets the voucher number.
func (v *VoucherBuilder) Number(num string) *VoucherBuilder {
	v.number = num
	return v
}

// Narration sets the voucher narration/description.
func (v *VoucherBuilder) Narration(narr string) *VoucherBuilder {
	v.narration = narr
	return v
}

// Debit adds a debit entry (amount is stored as negative per Tally convention).
func (v *VoucherBuilder) Debit(ledgerName string, amount float64) *VoucherBuilder {
	v.entries = append(v.entries, LedgerEntry{
		LedgerName:      ledgerName,
		IsDeemedPositive: true,
		Amount:          -amount, // Tally: debit amounts are negative
	})
	return v
}

// Credit adds a credit entry (amount is stored as positive per Tally convention).
func (v *VoucherBuilder) Credit(ledgerName string, amount float64) *VoucherBuilder {
	v.entries = append(v.entries, LedgerEntry{
		LedgerName:      ledgerName,
		IsDeemedPositive: false,
		Amount:          amount, // Tally: credit amounts are positive
	})
	return v
}

// Build generates the TALLYMESSAGE XML string containing the voucher.
func (v *VoucherBuilder) Build() string {
	var sb strings.Builder

	sb.WriteString(`<TALLYMESSAGE xmlns:UDF="TallyUDF">`)
	sb.WriteString(fmt.Sprintf(`<VOUCHER VCHTYPE="%s" ACTION="Create">`, v.voucherType))
	sb.WriteString(fmt.Sprintf(`<DATE>%s</DATE>`, v.date))
	sb.WriteString(fmt.Sprintf(`<VOUCHERTYPENAME>%s</VOUCHERTYPENAME>`, v.voucherType))

	if v.number != "" {
		sb.WriteString(fmt.Sprintf(`<VOUCHERNUMBER>%s</VOUCHERNUMBER>`, xmlEscape(v.number)))
	}
	if v.partyLedger != "" {
		sb.WriteString(fmt.Sprintf(`<PARTYLEDGERNAME>%s</PARTYLEDGERNAME>`, xmlEscape(v.partyLedger)))
	}
	if v.narration != "" {
		sb.WriteString(fmt.Sprintf(`<NARRATION>%s</NARRATION>`, xmlEscape(v.narration)))
	}

	for _, entry := range v.entries {
		deemed := "No"
		if entry.IsDeemedPositive {
			deemed = "Yes"
		}
		sb.WriteString(`<ALLLEDGERENTRIES.LIST>`)
		sb.WriteString(fmt.Sprintf(`<LEDGERNAME>%s</LEDGERNAME>`, xmlEscape(entry.LedgerName)))
		sb.WriteString(fmt.Sprintf(`<ISDEEMEDPOSITIVE>%s</ISDEEMEDPOSITIVE>`, deemed))
		sb.WriteString(fmt.Sprintf(`<AMOUNT>%.2f</AMOUNT>`, entry.Amount))
		sb.WriteString(`</ALLLEDGERENTRIES.LIST>`)
	}

	sb.WriteString(`</VOUCHER>`)
	sb.WriteString(`</TALLYMESSAGE>`)

	return sb.String()
}
