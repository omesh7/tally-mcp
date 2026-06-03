// Package tally provides an HTTP client for communicating with TallyPrime's XML server on port 9000.
//
// TallyPrime's XML server is single-threaded and expects UTF-16LE encoded XML payloads.
// This client handles encoding, decoding, connection management, and request serialization.
package tally

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Client communicates with TallyPrime's local XML HTTP server.
//
// It serializes requests through a mutex because Tally's port 9000
// is single-threaded — concurrent requests cause drops or freezes.
type Client struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex // Serializes requests to Tally's single-threaded XML server
}

// NewClient creates a TallyPrime HTTP client.
//
// Parameters:
//   - host: Tally server hostname (typically "localhost")
//   - port: Tally server port (typically 9000)
//   - timeout: HTTP request timeout
func NewClient(host string, port int, timeout time.Duration) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d/", host, port),
		httpClient: &http.Client{
			Timeout: timeout,
			// Disable keep-alive: Tally holds connections open indefinitely,
			// causing port exhaustion (WinError 10048) on Windows.
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}


// PostXML sends an XML request to TallyPrime and returns the decoded XML response.
//
// The request body is encoded to UTF-16LE (mandatory for Tally's C++ XML engine).
// The response is decoded from UTF-16LE back to UTF-8.
// Requests are serialized via mutex to prevent overloading Tally's single-threaded server.
func (c *Client) PostXML(ctx context.Context, xmlPayload string) (string, error) {
	// Serialize: Tally port 9000 is single-threaded
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create per-request encoder: golang.org/x/text transformers are stateful;
	// sharing a global instance across calls risks encoding corruption.
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	// Encode UTF-8 → UTF-16LE
	encoded, _, err := transform.Bytes(encoder, []byte(xmlPayload))
	if err != nil {
		return "", fmt.Errorf("utf-16le encoding failed: %w", err)
	}

	// Build HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(encoded))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml;charset=utf-16")
	req.Header.Set("Connection", "close") // CRITICAL: prevents port starvation on Windows
	req.ContentLength = int64(len(encoded))

	// Execute
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tally HTTP request failed (is TallyPrime running on %s?): %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	// Read raw response
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read tally response body: %w", err)
	}

	// Create per-request decoder: golang.org/x/text transformers are stateful.
	decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	// Decode UTF-16LE → UTF-8
	decoded, _, err := transform.Bytes(decoder, rawBody)
	if err != nil {
		// Fallback: try treating as raw UTF-8 (some Tally versions mix encodings)
		log.Printf("[TallyClient] UTF-16LE decode failed, falling back to raw bytes: %v", err)
		return string(rawBody), nil
	}

	return string(decoded), nil
}

// Ping checks if TallyPrime is reachable by sending a minimal XML request.
func (c *Client) Ping(ctx context.Context) error {
	// Minimal valid Tally XML that returns company list
	pingXML := `<ENVELOPE><HEADER><VERSION>1</VERSION><TALLYREQUEST>Export</TALLYREQUEST><TYPE>Data</TYPE><ID>R</ID></HEADER>
<BODY><DESC><STATICVARIABLES><SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT></STATICVARIABLES>
<TDL><TDLMESSAGE>
<REPORT NAME="R"><FORMS>F</FORMS></REPORT>
<FORM NAME="F"><PARTS>P</PARTS><XMLTAG>DATA</XMLTAG></FORM>
<PART NAME="P"><LINES>L</LINES><REPEAT>L : Col</REPEAT><SCROLLED>Vertical</SCROLLED></PART>
<LINE NAME="L"><FIELDS>FN</FIELDS><XMLTAG>ROW</XMLTAG></LINE>
<FIELD NAME="FN"><SET>$Name</SET><XMLTAG>Name</XMLTAG></FIELD>
<COLLECTION NAME="Col"><TYPE>Company</TYPE></COLLECTION>
</TDLMESSAGE></TDL></DESC></BODY></ENVELOPE>`

	_, err := c.PostXML(ctx, pingXML)
	return err
}
