# Architectural Design Specification: Go SRE Agent (Independent, A2A & Gemini Enterprise)

**Date**: 2026-09-11  
**Status**: Approved  
**Author**: SRE Platform & Antigravity  

---

## 1. Overview & Goals

The goal of this project is to build an autonomous Site Reliability Engineering (SRE) incident triage agent implemented in Go without depending on the Agent Development Kit (ADK) framework. 

The agent is designed to:
- Be 100% independent of ADK runtime and launcher packages.
- Ingest server logs and incident snippets, analyze error patterns and anomalies, classify incidents strictly by severity (`CRITICAL`, `WARNING`, `INFO`), and format standardized markdown postmortem reports.
- Implement the Agent-to-Agent (A2A) protocol over HTTP (JSON-RPC 2.0 + Agent Card discovery).
- Be deployable to Google Cloud **Agent Runtime** (Vertex AI Reasoning Engine) using `agents-cli`.
- Be testable locally via `agents-cli run --url <url> --mode a2a`.
- Be publishable and discoverable in **Gemini Enterprise** via A2A registration.

### Non-Goals
- Real-time cloud remediation (e.g. automatically issuing restarts or rollbacks in live clusters).
- Dependency on Python ADK or Go ADK runtime frameworks (`google.golang.org/adk/v2`).

---

## 2. Architecture & Components

### 2.1 Project Directory Structure

```
agent-no-adk/
├── cmd/
│   └── server/
│       └── main.go               # Server entrypoint: flags, port, env setup, HTTP server
├── internal/
│   ├── a2a/
│   │   ├── types.go              # A2A protocol JSON-RPC 2.0 types & AgentCard schema
│   │   ├── handler.go            # HTTP router for agent card, invocation, & health checks
│   │   └── handler_test.go       # Tests for A2A endpoints and JSON-RPC parsing
│   ├── agent/
│   │   ├── agent.go              # Agent core: GenAI client initialization, tool execution loop
│   │   ├── prompt.go             # SRE system instructions & postmortem rules
│   │   └── agent_test.go         # Tests for agent orchestration
│   └── tools/
│       ├── tools.go              # Gemini function declarations registry & dispatcher
│       ├── read_log.go           # 'read_log_file' tool (bounded lines/offset)
│       ├── parse_log.go          # 'parse_log_snippet' tool (pattern extraction, status codes)
│       ├── format_report.go      # 'format_incident_report' tool (postmortem markdown generator)
│       └── tools_test.go         # Unit tests for SRE tools and secret redaction
├── sample_logs/
│   ├── db_exhaustion.log         # Sample CRITICAL incident (pool saturation, 500s, deadlocks)
│   ├── gateway_timeout.log       # Sample WARNING incident (504 gateway timeout, latency)
│   └── normal_startup.log        # Sample INFO incident (routine startup & deprecations)
├── docs/
│   └── superpowers/specs/        # Specification documents
├── Dockerfile                    # Multi-stage container build for Agent Runtime
├── agents-cli-manifest.yaml      # Agent CLI metadata manifest
├── go.mod
└── go.sum
```

### 2.2 Go Module Dependencies
The project avoids ADK packages and depends only on:
- `google.golang.org/genai`: Official Google GenAI Go SDK for Gemini model inference and function calling.
- `github.com/google/uuid`: Unique identifier generation for JSON-RPC messages and sessions.
- Standard Library (`net/http`, `encoding/json`, `os`, `regexp`, `context`, etc.).

---

## 3. A2A Protocol Implementation

The service acts as an A2A protocol provider:

### 3.1 Endpoints

1. `GET /.well-known/agent-card.json` & `GET /a2a/sre-triage-agent/.well-known/agent-card.json`:
   - Returns the A2A `AgentCard` JSON payload.
   - Advertises capabilities, supported interfaces (`JSONRPC`), input/output mime types, and agent skills.
   - Dynamic URL resolution based on `Host` header or `APP_URL` environment variable.

2. `POST /a2a/invoke` (and `POST /`):
   - Handles JSON-RPC 2.0 method `message/send`.
   - Ingests user messages with context ID (session ID).
   - Returns standard JSON-RPC 2.0 response containing the agent message and formatted postmortem report.
   - Supports Server-Sent Events (SSE `data: {...}\n\n`) when requested or configured.

3. `GET /` & `GET /list-apps`:
   - Health check and app discovery endpoints for container readiness probes and `agents-cli` verification.

### 3.2 Wire Data Schemas

#### A2A Request (`message/send`)
```json
{
  "jsonrpc": "2.0",
  "id": "req-uuid",
  "method": "message/send",
  "params": {
    "message": {
      "role": "user",
      "parts": [{ "kind": "text", "text": "Triage sample_logs/db_exhaustion.log" }],
      "contextId": "session-123"
    }
  }
}
```

#### A2A Response
```json
{
  "jsonrpc": "2.0",
  "id": "req-uuid",
  "result": {
    "role": "agent",
    "parts": [{ "kind": "text", "text": "# SRE Incident Report\n..." }],
    "contextId": "session-123"
  }
}
```

---

## 4. SRE Agent & Function Calling

### 4.1 Model & Client
- **Model**: `gemini-3.8-flash` (override via `GEMINI_MODEL`).
- **Backend Selection**:
  - Automatically selects `genai.BackendVertexAI` when `GOOGLE_CLOUD_PROJECT` or `GOOGLE_GENAI_USE_VERTEXAI=true` is present, authenticating via GCP Application Default Credentials (ADC).
  - Falls back to `genai.BackendGeminiAPI` with `GEMINI_API_KEY` for local development.

### 4.2 SRE Tools

1. **`read_log_file`**:
   - Parameters: `file_path` (string, required), `max_lines` (int, default 200, max 1000), `offset` (int, default 0).
   - Safely reads lines within boundaries, returning line counts and offset indicators.
2. **`parse_log_snippet`**:
   - Parameters: `log_text` (string, required).
   - Analyzes raw logs and returns structured summary:
     - Log level counts (`CRITICAL`, `FATAL`, `ERROR`, `WARN`, `INFO`).
     - Extracted HTTP status codes (`500`, `502`, `503`, `504`) and gRPC codes.
     - Extracted unique exception names and stack trace roots.
     - Timestamp span (start to finish).
3. **`format_incident_report`**:
   - Parameters: `incident_id`, `title`, `severity` (`CRITICAL` | `WARNING` | `INFO`), `impacted_services`, `timeline`, `root_cause`, `mitigation_steps`, `preventative_actions`.
   - Enforces PII/secret masking (e.g. redacting `Bearer [REDACTED]`, `password=***`, `api_key=***`).
   - Renders GitHub-flavored Markdown postmortem conforming to SRE best practices.

### 4.3 Autonomous ReAct Loop
The agent runs an internal iterative loop:
1. Ingests user input with SRE persona instructions and tool definitions.
2. Invokes Gemini.
3. If Gemini returns `FunctionCalls`, executes the requested Go tools and appends `FunctionResponse` objects to the dialogue turn.
4. Loops until Gemini produces the final text response (or hits a maximum safety bound of 8 turns).

---

## 5. Deployment & Integration

### 5.1 Dockerfile (Agent Runtime Target)
- Multi-stage build (`golang:1.25` build stage $\rightarrow$ `gcr.io/distroless/static-debian12:nonroot`).
- Produces a minimal (~15MB), statically linked, secure binary.
- Copies `sample_logs/` so local file triage works out of the box.
- Listens on `PORT` (defaults to 8080).

### 5.2 Agent CLI Configuration (`agents-cli-manifest.yaml`)
```yaml
name: sre-triage-agent
acli_version: 1.5.0
agent_directory: .
region: us-central1
language: go
base_template: custom
create_params:
  deployment_target: agent_runtime
  is_a2a: true
  agent_gateway: false
```

### 5.3 Publishing to Gemini Enterprise
Using `agents-cli publish gemini-enterprise`:
```bash
agents-cli publish gemini-enterprise \
  --registration-type a2a \
  --agent-card-url "https://${LOCATION}-aiplatform.googleapis.com/reasoningEngines/v1/${ENGINE_ID}/api/.well-known/agent-card.json" \
  --gemini-enterprise-app-id "${GEMINI_ENTERPRISE_APP_ID}" \
  --display-name "SRE Incident Triage Agent" \
  --description "Autonomous incident postmortem and log triage assistant"
```

---

## 6. Verification Plan

### Automated Tests
1. `go test -v ./internal/tools/...`: Verifies `read_log_file`, `parse_log_snippet`, `format_incident_report`, and secret sanitization.
2. `go test -v ./internal/a2a/...`: Verifies agent card JSON generation, JSON-RPC 2.0 parsing, and HTTP routes.
3. `go test -v ./internal/agent/...`: Verifies prompt generation and function calling dispatcher.

### Local End-to-End Verification
1. Start server:
   ```bash
   go run ./cmd/server --port 8080
   ```
2. Verify agent card discovery:
   ```bash
   curl -s http://localhost:8080/.well-known/agent-card.json | jq .
   ```
3. Test using `agents-cli`:
   ```bash
   agents-cli run --url http://localhost:8080 --mode a2a "Triage the incident in sample_logs/db_exhaustion.log"
   ```
4. Verify severity classification and markdown report formatting on all three sample log scenarios (`db_exhaustion.log`, `gateway_timeout.log`, `normal_startup.log`).
