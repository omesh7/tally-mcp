# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.0] - 2026-06-03

### Added
* **Go Module Rewrite**: Ported entire codebase from Python prototype to a high-performance modular Go module (`github.com/omesh7/tally-mcp`).
* **Official MCP SDK Integration**: Integrated with the official Model Context Protocol Go SDK for reliable stdio JSON-RPC 2.0 communication.
* **Fluent XML & TDL Builder**: Introduced a composable builder (`internal/xml/`) replacing inline string manipulation.
* **Safe Request Serialization**: Added a global sync Mutex inside the HTTP transport wrapper to prevent Tally Prime single-threaded port 9000 crashes.
* **BOM & Response Sanitization**: Built an XML cleaning parser to strip UTF-8 Byte Order Marks (BOM), null bytes, and invalid character entities emitted by Tally.
* **Input Parameter Validation**: Added validations to ledger search and journal postings to reject malformed inputs locally before hitting Tally.
* **XML Escaping Security**: Implemented security escaping for user-supplied string arguments in both query payloads and static variables to prevent XML injection.
* **Unit Test Suite**: Created unit test suites under `internal/xml/` and `internal/tally/` to verify escaping, builders, cleaning, and case-insensitive parses.
* **Windows ASCII Build Tool**: Wrote `scripts/build.ps1` to easily compile the single dependency-free executable on Windows environments.

---
> [!NOTE]
> This is the initial open-source release (v0.1.0) of TallyMCP.
