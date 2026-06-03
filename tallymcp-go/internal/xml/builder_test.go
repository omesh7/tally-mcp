package tallyxml

import (
	"strings"
	"testing"
)

func TestXmlEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello World"},
		{"HDFC & Co", "HDFC &amp; Co"},
		{"<tag>", "&lt;tag&gt;"},
		{"\"double-quotes\"", "&quot;double-quotes&quot;"},
		{"'single-quotes'", "&apos;single-quotes&apos;"},
		{"A & B < C > D \" E ' F", "A &amp; B &lt; C &gt; D &quot; E &apos; F"},
	}

	for _, test := range tests {
		result := xmlEscape(test.input)
		if result != test.expected {
			t.Errorf("xmlEscape(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

func TestExportEnvelope(t *testing.T) {
	tdl := NewTDL().
		Report("MyReport", "MyForm").
		Form("MyForm", "MYTAG", "MyPart").
		Part("MyPart", "MyLine", "MyCollection").
		Line("MyLine", "MYROW", "F1", "F2").
		Field("F1", "Col1", "$Name").
		Field("F2", "Col2", "$Parent").
		Collection("MyCollection", CollectionLedger)

	envelope := NewExportEnvelope().WithTDL(tdl)
	xml := envelope.Build()

	// Verify header components
	if !strings.Contains(xml, "<TALLYREQUEST>Export</TALLYREQUEST>") {
		t.Errorf("ExportEnvelope missing TALLYREQUEST Export")
	}
	if !strings.Contains(xml, "<TYPE>Data</TYPE>") {
		t.Errorf("ExportEnvelope missing TYPE Data")
	}
	if !strings.Contains(xml, "<ID>R</ID>") {
		t.Errorf("ExportEnvelope missing ID R")
	}

	// Verify static variables
	if !strings.Contains(xml, "<STATICVARIABLES><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES>") {
		t.Errorf("ExportEnvelope missing or incorrect static variables")
	}

	// Verify TDL contents
	if !strings.Contains(xml, `<REPORT NAME="MyReport"><FORMS>MyForm</FORMS></REPORT>`) {
		t.Errorf("ExportEnvelope missing Report tag")
	}
	if !strings.Contains(xml, `<FORM NAME="MyForm"><PARTS>MyPart</PARTS><XMLTAG>MYTAG</XMLTAG></FORM>`) {
		t.Errorf("ExportEnvelope missing Form tag")
	}
	if !strings.Contains(xml, `<PART NAME="MyPart"><LINES>MyLine</LINES><REPEAT>MyLine : MyCollection</REPEAT>`) {
		t.Errorf("ExportEnvelope missing Part tag")
	}
	if !strings.Contains(xml, `<LINE NAME="MyLine"><FIELDS>F1,F2</FIELDS><XMLTAG>MYROW</XMLTAG></LINE>`) {
		t.Errorf("ExportEnvelope missing Line tag")
	}
}

func TestImportEnvelope(t *testing.T) {
	voucherXML := NewJournalVoucher("20260401").
		Debit("HDFC & Co", 1000).
		Credit("Capital Account", 1000).
		Narration("Test Entry").
		Build()

	envelope := NewImportEnvelope("All Masters").WithData(voucherXML)
	xml := envelope.Build()

	// Verify header components
	if !strings.Contains(xml, "<TALLYREQUEST>Import</TALLYREQUEST>") {
		t.Errorf("ImportEnvelope missing TALLYREQUEST Import")
	}
	if !strings.Contains(xml, "<ID>All Masters</ID>") {
		t.Errorf("ImportEnvelope missing ID All Masters")
	}

	// Verify data section
	if !strings.Contains(xml, "<DATA>") || !strings.Contains(xml, "</DATA>") {
		t.Errorf("ImportEnvelope missing DATA tags")
	}
	if !strings.Contains(xml, "HDFC &amp; Co") {
		t.Errorf("ImportEnvelope missing escaped ledger name HDFC &amp; Co")
	}
}

func TestVoucherBuilder(t *testing.T) {
	builder := NewJournalVoucher("20260402").
		Number("JV-01").
		Narration("Rent for \"Office Space\" & parking").
		Debit("Rent Expense", 5000).
		Credit("Bank A/c", 5000)

	xml := builder.Build()

	// Verify XML structure
	if !strings.Contains(xml, `xmlns:UDF="TallyUDF"`) {
		t.Errorf("VoucherBuilder output missing namespace declaration")
	}
	if !strings.Contains(xml, `<VOUCHER VCHTYPE="Journal" ACTION="Create">`) {
		t.Errorf("VoucherBuilder output missing VOUCHER type/action tags")
	}
	if !strings.Contains(xml, `<DATE>20260402</DATE>`) {
		t.Errorf("VoucherBuilder output missing date")
	}
	if !strings.Contains(xml, `<VOUCHERNUMBER>JV-01</VOUCHERNUMBER>`) {
		t.Errorf("VoucherBuilder output missing voucher number")
	}
	if !strings.Contains(xml, `<NARRATION>Rent for &quot;Office Space&quot; &amp; parking</NARRATION>`) {
		t.Errorf("VoucherBuilder output missing/incorrect escaped narration")
	}

	// Verify entries and amount signs (Debit must be negative, Credit must be positive)
	if !strings.Contains(xml, "<LEDGERNAME>Rent Expense</LEDGERNAME>") {
		t.Errorf("VoucherBuilder output missing Rent Expense entry")
	}
	if !strings.Contains(xml, "<AMOUNT>-5000.00</AMOUNT>") {
		t.Errorf("VoucherBuilder output missing debit negative amount (-5000.00)")
	}
	if !strings.Contains(xml, "<ISDEEMEDPOSITIVE>Yes</ISDEEMEDPOSITIVE>") {
		t.Errorf("VoucherBuilder output missing ISDEEMEDPOSITIVE Yes for debit")
	}

	if !strings.Contains(xml, "<LEDGERNAME>Bank A/c</LEDGERNAME>") {
		t.Errorf("VoucherBuilder output missing Bank A/c entry")
	}
	if !strings.Contains(xml, "<AMOUNT>5000.00</AMOUNT>") {
		t.Errorf("VoucherBuilder output missing credit positive amount (5000.00)")
	}
	if !strings.Contains(xml, "<ISDEEMEDPOSITIVE>No</ISDEEMEDPOSITIVE>") {
		t.Errorf("VoucherBuilder output missing ISDEEMEDPOSITIVE No for credit")
	}
}

func TestEqualFilter(t *testing.T) {
	filter := EqualFilter("Name", "HDFC & Bank")
	expected := `$$IsEqual:$Name:"HDFC &amp; Bank"`
	if filter != expected {
		t.Errorf("EqualFilter = %q; want %q", filter, expected)
	}
}
