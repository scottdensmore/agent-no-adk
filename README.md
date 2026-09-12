# SRE Incident Triage Agent (Go / No-ADK)

An autonomous Site Reliability Engineering (SRE) incident triage agent built natively in **Go** using the official Google GenAI SDK (`google.golang.org/genai`) targeting `gemini-3.8-flash`.

This project implements the **Agent-to-Agent (A2A)** protocol and **ADK-compatible SSE streaming** without taking dependencies on Google's Agent Development Kit (ADK) runtime libraries. It works seamlessly with the `agents-cli` toolchain, Google Cloud Agent Runtime, and Gemini Enterprise.

---

## Architecture & Features

* **Autonomous ReAct Loop**: Executes iterative function-calling turns with Gemini 3.8 Flash to inspect logs, gather evidence, and format postmortems.
* **Native SRE Tools**:
  * `read_log_file`: Memory-efficient streaming reader with offset and line limits.
  * `parse_log_snippet`: Extracts log levels, error frequencies, HTTP/gRPC codes, and services.
  * `format_incident_report`: Generates structured markdown postmortems and redacts sensitive credentials (`[REDACTED]`).
* **Multi-Protocol HTTP Server**:
  * **A2A v1.0**: JSON-RPC 2.0 (`message/send`) and Agent Card discovery (`/.well-known/agent-card.json`).
  * **ADK SSE Streaming**: Live event streaming (`/run_sse`) for real-time tool inspection.
  * **Built-in Web Playground**: Browser interface (`/chat`) with an interactive **Side Panel** showing live Tool Executions, Event Streams, and Telemetry.
* **Enterprise Observability**: Distributed tracing with Google Cloud Trace (OpenTelemetry) and structured JSON logging.
* **Dual Auth Backends**: Auto-detects Google Cloud Vertex AI (ADC / Workload Identity) or Google AI Studio (`GEMINI_API_KEY`).
* **Container Ready**: Multi-stage distroless Docker image running as an unprivileged user (`nonroot`).

---

## Project Structure

```text
.
├── cmd/
│   └── server/             # Application entrypoint & HTTP server
├── internal/
│   ├── a2a/                # A2A JSON-RPC 2.0, Agent Card, and ADK SSE handler
│   ├── agent/              # Gemini ReAct loop, system persona & prompt
│   ├── telemetry/          # OpenTelemetry Cloud Trace & GCP structured logging
│   └── tools/              # SRE tools (read_log_file, parse_log_snippet, format_report)
├── sample_logs/            # Test incident fixtures (db_exhaustion, gateway_timeout, etc.)
├── tests/
│   └── eval/               # Evaluation datasets and configuration for agents-cli eval
├── Dockerfile              # Distroless multi-stage container build
├── agents-cli-manifest.yaml# Manifest for Agent Runtime deployment
└── go.mod
```

---

## Prerequisites

* **Go**: 1.25+
* **agents-cli**: Google Agents CLI (`uv tool install google-agents-cli` or `pip install google-agents-cli`)
* **Google Cloud Auth**:
  * For Vertex AI: Application Default Credentials (`gcloud auth application-default login`) and a configured project (`export GOOGLE_CLOUD_PROJECT="YOUR_PROJECT_ID"`).
  * For Google AI Studio: API key (`export GEMINI_API_KEY="YOUR_API_KEY"`).

---

## Local Development

### 1. Build and Run the Server

```bash
# Build binary
go build -o bin/server ./cmd/server

# Run server on port 8080 (defaults to gemini-3.8-flash)
PORT=8080 ./bin/server
```

### 2. Verify Discovery

```bash
curl -s http://localhost:8080/.well-known/agent-card.json | jq .
```

### 3. Web Playground & Inspector

Open your browser to:
**`http://localhost:8080/chat`** *(or `http://localhost:8080/`)*

* **Chat Pane**: Interactive triage prompt interface with quick test scenarios.
* **Side Panel — 🛠️ Tool Calls**: Real-time cards showing tool names, arguments, status badges (`RUNNING` ➔ `SUCCESS`), and outputs.
* **Side Panel — ⚡ Events**: Live SSE event stream from `/run_sse`.
* **Side Panel — 📊 Telemetry**: Live session ID, status, and Cloud Trace links.

---

## Using with `agents-cli`

### 1. Querying with Live Tool Tracing (`--mode adk`)

Use `--mode adk` to connect to the agent's `/run_sse` endpoint. This prints each tool execution and response in real time as the agent reasons:

```bash
agents-cli run --mode adk --url http://localhost:8080 "Triage the incident in sample_logs/db_exhaustion.log"
```

Output example:
```text
Querying remote agent: http://localhost:8080 (mode: adk)
[user]: Triage the incident in sample_logs/db_exhaustion.log

[tool_call: read_log_file({"file_path": "sample_logs/db_exhaustion.log", "max_lines": 200, "offset": 0})]
[tool_response: read_log_file -> {"lines": [...], "lines_read": 6, "total_lines": 6}]

[tool_call: parse_log_snippet({"log_text": "..."})]
[tool_response: parse_log_snippet -> {"exceptions": ["ConnectionPoolTimeoutException", "OutOfMemoryError"], ...}]

[tool_call: format_incident_report({"impacted_services": [...], "severity": "CRITICAL", ...})]
[tool_response: format_incident_report -> {"report": "# SRE Incident Report: INC-..."}]

# SRE Incident Report: INC-20260911-01 - Database Connection Pool Exhaustion
...
```

* **Verbose Mode (`-v`)**: Add `-v` to print the full raw JSON payload for every event frame:
  ```bash
  agents-cli run --mode adk -v --url http://localhost:8080 "Triage the incident in sample_logs/gateway_timeout.log"
  ```
* **Session Continuity (`--session-id`)**: Pass `--session-id <id>` to continue a multi-turn conversation:
  ```bash
  agents-cli run --mode adk --url http://localhost:8080 --session-id "YOUR_SESSION_ID" "What immediate actions should I take first?"
  ```

### 2. Querying via A2A Protocol (`--mode a2a`)

Use `--mode a2a` to query the agent via the A2A JSON-RPC 2.0 endpoint (`/a2a/invoke`):

```bash
agents-cli run --mode a2a --url http://localhost:8080 "What are your SRE triage capabilities?"
```

### 3. Running Automated Evaluations (`agents-cli eval`)

Run quality and safety evaluation suites against the agent using the provided eval fixtures:

```bash
agents-cli eval run \
  --dataset tests/eval/datasets/sre-eval.json \
  --config tests/eval/eval_config.yaml \
  --url http://localhost:8080 \
  --mode adk
```

---

## Deployment

### Deploying to Agent Runtime

The repository includes an `agents-cli-manifest.yaml` targeting Google Cloud Agent Runtime:

```bash
agents-cli deploy \
  --project YOUR_PROJECT_ID \
  --region us-central1
```

### Registering with Gemini Enterprise

Once deployed, publish the agent to your Gemini Enterprise application:

```bash
agents-cli publish gemini-enterprise \
  --project-id YOUR_PROJECT_ID \
  --agent-runtime-id projects/YOUR_PROJECT_NUMBER/locations/us-central1/reasoningEngines/YOUR_REASONING_ENGINE_ID \
  --gemini-enterprise-app-id projects/YOUR_PROJECT_NUMBER/locations/global/collections/default_collection/engines/YOUR_APP_ID \
  --display-name "Scott SRE Core Agent" \
  --description "Autonomous SRE incident triage assistant for log parsing, anomaly classification, and postmortem generation." \
  --tool-description "Parses server logs, diagnoses failures, classifies incident severity, and formats SRE triage incident reports." \
  --registration-type adk
```

---

## HTTP Endpoints Reference

| Endpoint | Method | Protocol | Description |
| :--- | :---: | :---: | :--- |
| `/.well-known/agent-card.json` | `GET` | HTTP | A2A Agent Card specification |
| `/a2a/invoke` | `POST` | JSON-RPC 2.0 | A2A `message/send` execution |
| `/run_sse` | `POST` | SSE / ADK | Real-time event and tool call streaming |
| `/apps/{app}/app-info` | `GET` | REST | Agent discovery and metadata |
| `/chat` or `/` | `GET` | HTML | Web Playground & Real-time Inspector UI |
| `/` | `GET` | JSON | Service health check |

---

## Running Tests

```bash
# Run all unit and integration tests
go test -v ./...

# Run with race detector
go test -race ./...
```
