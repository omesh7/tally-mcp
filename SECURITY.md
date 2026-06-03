# Security Policy & Audit

We take the security of your accounting data and system extremely seriously. Because TallyMCP manages direct access to Tally Prime databases, it is designed around strict security boundaries.

---

## 🛡️ Security Model

### 1. Local-Only Boundary
TallyMCP runs entirely as a **local process** on your machine.
* It communicates with your AI client (e.g. Claude Desktop) over standard input/output (STDIO) pipes.
* It communicates with Tally Prime over local loopback (`http://localhost:9000`).
* **Zero Network Calls**: TallyMCP does not call external APIs, send telemetry, or store data in the cloud. It operates strictly within your machine's boundary, keeping you fully compliant with the **Digital Personal Data Protection (DPDP) Act, 2023**.

### 2. Input Hardening
* All dynamic requests are fully sanitized and hardened using industry best practices to prevent command injection or formatting corruption.
* Static query variables are also safely handled under the same strict checks.

### 3. Request Serialization
* Tally Prime's local engine processes requests sequentially. TallyMCP automatically serializes all outgoing requests to prevent conflicts, ensuring stable communication.

---

## ⚠️ Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it privately.

Please email vulnerability reports to:
* **Email**: omeshv845@gmail.com (or contact via GitHub profile: [omesh7](https://github.com/omesh7))

Please include a description of the issue and replication steps. We will acknowledge receipt of your report within 48 hours.
