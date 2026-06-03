package tallyxml

import (
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// TDL Builder — Builds the <TDL><TDLMESSAGE>...</TDLMESSAGE></TDL> block
// ──────────────────────────────────────────────────────────────────────────────

// TDL builds the TDL/TDLMESSAGE block used inside export query envelopes.
// It holds all report, form, part, line, field, collection, and filter definitions.
type TDL struct {
	parts []string // Accumulated XML fragments
}

// NewTDL creates a new TDL builder.
func NewTDL() *TDL {
	return &TDL{}
}

// Report adds a REPORT definition: <REPORT NAME="name"><FORMS>formName</FORMS></REPORT>
func (t *TDL) Report(name, formName string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<REPORT NAME="%s"><FORMS>%s</FORMS></REPORT>`, name, formName))
	return t
}

// Form adds a FORM definition: <FORM NAME="name"><PARTS>partName</PARTS><XMLTAG>tag</XMLTAG></FORM>
func (t *TDL) Form(name, xmlTag, partName string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<FORM NAME="%s"><PARTS>%s</PARTS><XMLTAG>%s</XMLTAG></FORM>`, name, partName, xmlTag))
	return t
}

// Part adds a PART definition with vertical scrolling and repeat.
func (t *TDL) Part(name, lineName, collectionName string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<PART NAME="%s"><LINES>%s</LINES><REPEAT>%s : %s</REPEAT><SCROLLED>Vertical</SCROLLED></PART>`,
		name, lineName, lineName, collectionName))
	return t
}

// Line adds a LINE definition with specified fields and XML tag.
func (t *TDL) Line(name, xmlTag string, fieldNames ...string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<LINE NAME="%s"><FIELDS>%s</FIELDS><XMLTAG>%s</XMLTAG></LINE>`,
		name, strings.Join(fieldNames, ","), xmlTag))
	return t
}

// Field adds a FIELD definition: <FIELD NAME="name"><SET>formula</SET><XMLTAG>tag</XMLTAG></FIELD>
func (t *TDL) Field(name, xmlTag, formula string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<FIELD NAME="%s"><SET>%s</SET><XMLTAG>%s</XMLTAG></FIELD>`, name, formula, xmlTag))
	return t
}

// Collection adds a COLLECTION definition with optional filter and fetch.
func (t *TDL) Collection(name, collType string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<COLLECTION NAME="%s"><TYPE>%s</TYPE></COLLECTION>`, name, collType))
	return t
}

// CollectionWithFilter adds a COLLECTION with a FILTER reference.
func (t *TDL) CollectionWithFilter(name, collType, filterName string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<COLLECTION NAME="%s"><TYPE>%s</TYPE><FILTER>%s</FILTER></COLLECTION>`,
		name, collType, filterName))
	return t
}

// CollectionWithFetch adds a COLLECTION with a FETCH clause (for sub-collections like AllLedgerEntries).
func (t *TDL) CollectionWithFetch(name, collType, fetchField string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<COLLECTION NAME="%s"><TYPE>%s</TYPE><FETCH>%s</FETCH></COLLECTION>`,
		name, collType, fetchField))
	return t
}

// Filter adds a SYSTEM filter formula: <SYSTEM TYPE="Formula" NAME="name">formula</SYSTEM>
func (t *TDL) Filter(name, formula string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<SYSTEM TYPE="Formula" NAME="%s">%s</SYSTEM>`, name, formula))
	return t
}

// FilterFormulae adds a SYSTEM filter with TYPE="Formulae" (plural — required for some Tally queries).
func (t *TDL) FilterFormulae(name, formula string) *TDL {
	t.parts = append(t.parts, fmt.Sprintf(
		`<SYSTEM TYPE="Formulae" NAME="%s">%s</SYSTEM>`, name, formula))
	return t
}

// Build generates the complete <TDL><TDLMESSAGE>...</TDLMESSAGE></TDL> block.
func (t *TDL) Build() string {
	var sb strings.Builder
	sb.WriteString("<TDL><TDLMESSAGE>")
	for _, part := range t.parts {
		sb.WriteString(part)
	}
	sb.WriteString("</TDLMESSAGE></TDL>")
	return sb.String()
}
