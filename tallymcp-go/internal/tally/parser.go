package tally

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// invalidCharRefRegex matches invalid XML character references like &#NNN; that Tally sometimes emits.
var invalidCharRefRegex = regexp.MustCompile(`&#\d+;`)

// CleanXML sanitizes raw Tally XML response text for safe parsing.
//
// Tally's XML responses contain several problematic patterns:
//   - BOM markers (U+FEFF)
//   - Null bytes (\x00)
//   - Invalid character references (&#NNN;)
//   - Double carriage returns (\r\r\n)
func CleanXML(raw string) string {
	s := raw

	// Strip BOM
	s = strings.TrimPrefix(s, "\uFEFF")

	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")

	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\r\n", "\n")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Remove invalid character references
	s = invalidCharRefRegex.ReplaceAllString(s, "")

	return strings.TrimSpace(s)
}

// GenericElement represents a generic XML element with case-preserved tag name,
// attributes, text content, and child elements.
//
// This is used instead of encoding/xml's strict struct-based unmarshalling
// because Tally returns tags in inconsistent casing (NAME vs Name vs name).
type GenericElement struct {
	XMLName  xml.Name
	Attrs    []xml.Attr       `xml:",any,attr"`
	Content  string           `xml:",chardata"`
	Children []*GenericElement `xml:",any"`
}

// ParseXML parses cleaned Tally XML into a generic element tree.
func ParseXML(raw string) (*GenericElement, error) {
	cleaned := CleanXML(raw)
	var root GenericElement
	if err := xml.Unmarshal([]byte(cleaned), &root); err != nil {
		return nil, fmt.Errorf("xml parse error: %w", err)
	}
	return &root, nil
}

// FindAll returns all descendant elements matching the given tag name (case-insensitive).
func (e *GenericElement) FindAll(tagName string) []*GenericElement {
	var results []*GenericElement
	upper := strings.ToUpper(tagName)
	e.findAllRecursive(upper, &results)
	return results
}

func (e *GenericElement) findAllRecursive(upperTag string, results *[]*GenericElement) {
	for _, child := range e.Children {
		if strings.ToUpper(child.XMLName.Local) == upperTag {
			*results = append(*results, child)
		}
		child.findAllRecursive(upperTag, results)
	}
}

// FindText returns the text content of the first child element matching tagName (case-insensitive).
// Returns empty string if not found.
func (e *GenericElement) FindText(tagName string) string {
	upper := strings.ToUpper(tagName)
	for _, child := range e.Children {
		if strings.ToUpper(child.XMLName.Local) == upper {
			return strings.TrimSpace(child.Content)
		}
	}
	return ""
}

// FindTextDeep searches all descendants (not just direct children) for the first
// element matching tagName (case-insensitive) and returns its text content.
func (e *GenericElement) FindTextDeep(tagName string) string {
	upper := strings.ToUpper(tagName)
	return e.findTextDeepRecursive(upper)
}

func (e *GenericElement) findTextDeepRecursive(upperTag string) string {
	for _, child := range e.Children {
		if strings.ToUpper(child.XMLName.Local) == upperTag {
			return strings.TrimSpace(child.Content)
		}
		if result := child.findTextDeepRecursive(upperTag); result != "" {
			return result
		}
	}
	return ""
}

// Attr returns the value of the named attribute (case-insensitive), or empty string.
func (e *GenericElement) Attr(name string) string {
	upper := strings.ToUpper(name)
	for _, a := range e.Attrs {
		if strings.ToUpper(a.Name.Local) == upper {
			return a.Value
		}
	}
	return ""
}
