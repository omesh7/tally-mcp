package tally

import (
	"testing"
)

func TestCleanXML(t *testing.T) {
	input := "\uFEFF<RESPONSE>\x00\r\r\n  <NAME>HDFC&#38;Co</NAME>\r\n  <VALUE>&#123;Test&#125;</VALUE>\r</RESPONSE>"
	expected := "<RESPONSE>\n  <NAME>HDFCCo</NAME>\n  <VALUE>Test</VALUE>\n</RESPONSE>"

	result := CleanXML(input)
	if result != expected {
		t.Errorf("CleanXML output mismatch.\nGot:  %q\nWant: %q", result, expected)
	}
}

func TestParseXMLAndGenericElement(t *testing.T) {
	rawXML := `
<ENVELOPE>
  <HEADER>
    <STATUS>1</STATUS>
  </HEADER>
  <BODY>
    <DATA>
      <ROW id="1">
        <NAME>Company A</NAME>
        <PARENT>Group A</PARENT>
      </ROW>
      <ROW id="2">
        <NAME>Company B</NAME>
        <PARENT>Group B</PARENT>
      </ROW>
    </DATA>
  </BODY>
</ENVELOPE>
`

	root, err := ParseXML(rawXML)
	if err != nil {
		t.Fatalf("ParseXML failed: %v", err)
	}

	// Test case-insensitive direct text find
	headerEl := root.Children[0]
	if headerEl == nil || headerEl.XMLName.Local != "HEADER" {
		t.Errorf("Could not get header element")
	}
	status := headerEl.FindText("status")
	if status != "1" {
		t.Errorf("headerEl.FindText('status') = %q, want '1'", status)
	}

	// Test FindAll (case-insensitive)
	rows := root.FindAll("row")
	if len(rows) != 2 {
		t.Errorf("FindAll('row') returned %d elements, want 2", len(rows))
	}

	// Test Attr (case-insensitive) on rows
	if rows[0].Attr("id") != "1" {
		t.Errorf("row[0].Attr('id') = %q, want '1'", rows[0].Attr("id"))
	}
	if rows[1].Attr("ID") != "2" {
		t.Errorf("row[1].Attr('ID') = %q, want '2'", rows[1].Attr("ID"))
	}

	// Test FindText (case-insensitive direct child)
	name0 := rows[0].FindText("name")
	if name0 != "Company A" {
		t.Errorf("rows[0].FindText('name') = %q, want 'Company A'", name0)
	}

	parent1 := rows[1].FindText("Parent")
	if parent1 != "Group B" {
		t.Errorf("rows[1].FindText('Parent') = %q, want 'Group B'", parent1)
	}

	// Test FindTextDeep
	deepName := root.FindTextDeep("NAME")
	if deepName != "Company A" {
		t.Errorf("FindTextDeep('NAME') = %q, want 'Company A'", deepName)
	}
}
