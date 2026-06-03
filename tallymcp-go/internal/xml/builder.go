// Package tallyxml provides a fluent, composable XML builder for TallyPrime XML API requests.
//
// Instead of writing raw XML strings (fragile, unreadable, error-prone),
// this package lets you build correct Tally XML programmatically:
//
//	xml := tallyxml.NewExportEnvelope().
//	    WithTDL(
//	        tallyxml.NewTDL().
//	            Report("R", "F").
//	            Form("F", "DATA", "P").
//	            Part("P", "L", "Col").
//	            Line("L", "ROW", "FN", "FP").
//	            Field("FN", "Name", "$Name").
//	            Field("FP", "Parent", "$Parent").
//	            Collection("Col", CollectionLedger, ""),
//	    ).
//	    Build()
package tallyxml

import (
	"fmt"
	"strings"
)

// xmlEscape escapes special XML characters in a string.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// ──────────────────────────────────────────────────────────────────────────────
// Envelope Builder
// ──────────────────────────────────────────────────────────────────────────────

// Envelope represents a complete Tally XML request envelope.
type Envelope struct {
	requestType string // "Export" or "Import"
	dataType    string // "Data"
	reportID    string // Report ID for exports (e.g. "R")
	importID    string // Import target (e.g. "All Masters", "All Vouchers")
	staticVars  map[string]string
	tdlContent  string // TDL block for export queries
	dataContent string // DATA block for import operations
}

// NewExportEnvelope creates an envelope for reading data from Tally.
func NewExportEnvelope() *Envelope {
	return &Envelope{
		requestType: "Export",
		dataType:    "Data",
		reportID:    "R",
		staticVars: map[string]string{
			"SVEXPORTFORMAT": "$$SysName:XML",
		},
	}
}

// NewImportEnvelope creates an envelope for writing data to Tally.
func NewImportEnvelope(importID string) *Envelope {
	return &Envelope{
		requestType: "Import",
		dataType:    "Data",
		importID:    importID,
		staticVars: map[string]string{
			"SVCurrentCompany": "##SVCurrentCompany",
		},
	}
}

// WithStaticVar adds a static variable to the envelope.
func (e *Envelope) WithStaticVar(name, value string) *Envelope {
	if e.staticVars == nil {
		e.staticVars = make(map[string]string)
	}
	e.staticVars[name] = value
	return e
}

// WithTDL sets the TDL content block (for export queries).
func (e *Envelope) WithTDL(tdl *TDL) *Envelope {
	e.tdlContent = tdl.Build()
	return e
}

// WithData sets the DATA content block (for import operations).
func (e *Envelope) WithData(data string) *Envelope {
	e.dataContent = data
	return e
}

// Build generates the complete XML string.
func (e *Envelope) Build() string {
	var sb strings.Builder

	sb.WriteString("<ENVELOPE>")

	// HEADER
	sb.WriteString("<HEADER>")
	sb.WriteString("<VERSION>1</VERSION>")
	sb.WriteString(fmt.Sprintf("<TALLYREQUEST>%s</TALLYREQUEST>", e.requestType))
	sb.WriteString(fmt.Sprintf("<TYPE>%s</TYPE>", e.dataType))
	if e.requestType == "Export" {
		sb.WriteString(fmt.Sprintf("<ID>%s</ID>", e.reportID))
	} else {
		sb.WriteString(fmt.Sprintf("<ID>%s</ID>", e.importID))
	}
	sb.WriteString("</HEADER>")

	// BODY
	sb.WriteString("<BODY>")

	// DESC with static variables
	sb.WriteString("<DESC>")
	if len(e.staticVars) > 0 {
		sb.WriteString("<STATICVARIABLES>")
		for k, v := range e.staticVars {
			sb.WriteString(fmt.Sprintf("<%s>%s</%s>", k, xmlEscape(v), k))
		}
		sb.WriteString("</STATICVARIABLES>")
	}

	// TDL block (export queries)
	if e.tdlContent != "" {
		sb.WriteString(e.tdlContent)
	}
	sb.WriteString("</DESC>")

	// DATA block (import operations)
	if e.dataContent != "" {
		sb.WriteString("<DATA>")
		sb.WriteString(e.dataContent)
		sb.WriteString("</DATA>")
	}

	sb.WriteString("</BODY>")
	sb.WriteString("</ENVELOPE>")

	return sb.String()
}
