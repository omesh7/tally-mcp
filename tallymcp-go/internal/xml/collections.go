package tallyxml

// ──────────────────────────────────────────────────────────────────────────────
// Collection Type Constants
// ──────────────────────────────────────────────────────────────────────────────

// Standard Tally collection types used in TDL queries.
const (
	CollectionCompany     = "Company"
	CollectionLedger      = "Ledger"
	CollectionStockItem   = "StockItem"
	CollectionStockGroup  = "StockGroup"
	CollectionVoucher     = "Voucher"
	CollectionVoucherType = "VoucherType"
	CollectionUnit        = "Unit"
	CollectionGodown      = "Godown"
	CollectionCostCentre  = "CostCentre"
	CollectionCurrency    = "Currency"
)

// ──────────────────────────────────────────────────────────────────────────────
// Common TDL Formulas
// ──────────────────────────────────────────────────────────────────────────────

// Standard TDL field formulas used across multiple tools.
const (
	FormulaName           = "$Name"
	FormulaParent         = "$Parent"
	FormulaClosingBalance = "$$NumValue:$ClosingBalance"
	FormulaIsDebit        = `if $$IsDebit:$ClosingBalance then "Dr" else "Cr"`
	FormulaIsDebitFull    = `if $$IsDebit:$ClosingBalance then "Debit" else "Credit"`
	FormulaBaseUnits      = "$BaseUnits"
	FormulaOpeningQty     = "$$NumValue:$OpeningBalance"
	FormulaOpeningValue   = "$$NumValue:$OpeningValue"
	FormulaVoucherNumber  = "$VoucherNumber"
	FormulaDate           = "$Date"
	FormulaCurrentCompany = "##SVCurrentCompany"
)

// EqualFilter builds a TDL filter formula: $$IsEqual:$fieldName:"value"
func EqualFilter(fieldName, value string) string {
	return `$$IsEqual:$` + fieldName + `:"` + xmlEscape(value) + `"`
}
