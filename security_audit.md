# TallyMCP Security Audit Report

**Date**: 2026-06-03  
**Status**: 🛡️ VERIFIED SAFE (No high or medium severity vulnerabilities found)  
**Target Codebase**: `github.com/omesh7/tally-mcp` (Go modules version)

---

## 1. Executive Summary

TallyMCP is a Model Context Protocol (MCP) server bridging AI clients to local Tally Prime instances. Because accounting data is highly sensitive and Tally Prime has write capabilities, the security model must enforce strict boundaries. 

This audit confirms that TallyMCP maintains a **local-first, isolated security model** with robust sanitization of LLM-supplied variables.

---

## 2. Threat Modeling & Vulnerability Analysis

### Threat 1: XML/TDL Injection (Mitigated)
* **Risk**: LLMs or malicious prompts injecting custom XML tags (e.g. `<ACTION>Delete</ACTION>` or injecting new ledger loops) to extract data or execute unauthorized writes in Tally.
* **Audit**:
  - All dynamic inputs inserted into XML envelopes and collections are strictly filtered through the `xmlEscape` utility function in `internal/xml/builder.go`.
  - The `xmlEscape` function replaces the five XML special characters:
    * `&` $\rightarrow$ `&amp;`
    * `<` $\rightarrow$ `&lt;`
    * `>` $\rightarrow$ `&gt;`
    * `"` $\rightarrow$ `&quot;`
    * `'` $\rightarrow$ `&apos;`
  - Handlers using static variables (such as target date or target company) escape and clean values before inclusion.
* **Result**: **PASS**. No raw unescaped strings are concatenated directly into XML structures.

### Threat 2: Remote Exposure & Network Leakage (Mitigated)
* **Risk**: Exposing Tally's single-threaded, unauthenticated TCP port `9000` to the external network.
* **Audit**:
  - TallyMCP does **not** bind to any network sockets or listen on any ports.
  - It utilizes standard stdin/stdout pipe streams (stdio transport) to communicate with the host AI process (e.g. Claude Desktop).
  - Outgoing connections from TallyMCP are hardcoded to `localhost` (`127.0.0.1`) only, preventing SSRF or DNS rebinding attacks.
* **Result**: **PASS**. The attack surface is restricted entirely to the local machine.

### Threat 3: Input Validation & Shell Injection (Mitigated)
* **Risk**: Parameter manipulation on write operations causing shell command execution or invalid transactions.
* **Audit**:
  - TallyMCP does not invoke subprocesses or execute shell commands (`os/exec` is not imported in the tool handlers).
  - Strong Go type definitions (e.g. `float64` for amounts, `int` for days) enforce validation at the JSON-RPC parser layer.
  - Custom validations reject negative amounts or empty fields before any XML is compiled.
* **Result**: **PASS**.

### Threat 4: Concurrency & Denial of Service (Mitigated)
* **Risk**: Concurrent queries from the AI server causing Tally Prime's single-threaded C++ XML parser to freeze or exhaust ports.
* **Audit**:
  - A global `sync.Mutex` in `internal/tally/client.go` serializes all requests, ensuring Tally Prime receives only one request at a time.
  - `DisableKeepAlives` is set to `true` on the HTTP transport to prevent socket exhaustion issues on Windows.
* **Result**: **PASS**.

---

## 3. Concluding Recommendations

TallyMCP is safe for production use in local environments. Users should follow standard system administration best practices:
1. Ensure the Windows machine running Tally Prime is protected by a local firewall.
2. Run Claude Desktop/Antigravity IDE and TallyMCP under standard (non-administrative) user privileges.
