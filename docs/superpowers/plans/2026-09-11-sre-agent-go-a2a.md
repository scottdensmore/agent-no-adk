# SRE Agent in Go (Independent, A2A & Gemini Enterprise) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an autonomous SRE incident triage agent in Go that operates independently of ADK, serves the A2A protocol over HTTP, deploys to Agent Runtime via `agents-cli`, and publishes to Gemini Enterprise.

**Architecture:** A standard Go service using `google.golang.org/genai` for `gemini-3.8-flash` function calling, an in-house A2A HTTP JSON-RPC 2.0 handler (`/.well-known/agent-card.json` and `/a2a/invoke`), an autonomous tool-calling loop, and native SRE tools (`read_log_file`, `parse_log_snippet`, `format_incident_report`).

**Tech Stack:** Go 1.25, `google.golang.org/genai`, `github.com/google/uuid`, Docker multi-stage build, `agents-cli` 1.5.0.

**Spec:** `docs/superpowers/specs/2026-09-11-sre-agent-go-a2a-design.md`

## Global Constraints

- Must NOT depend on `google.golang.org/adk` or Python ADK runtime libraries.
- Model default: `gemini-3.8-flash`.
- Backend support: Auto-detect Vertex AI with ADC (for Agent Runtime) vs Google AI Studio with `GEMINI_API_KEY` (for local development).
- A2A endpoints: Must serve both `/.well-known/agent-card.json` and `/a2a/sre-triage-agent/.well-known/agent-card.json`.
- A2A JSON-RPC method: `message/send` with context ID tracking.
- Health checks: Must serve `GET /` and `GET /list-apps`.
- Strict severity classifications: `CRITICAL`, `WARNING`, `INFO`.
- Guardrails: Mask tokens, passwords, and API keys with `[REDACTED]`.

---

### Task 1: Go Module Setup & Sample Logs

**Files:**
- Create: `go.mod`
- Create: `sample_logs/db_exhaustion.log`
- Create: `sample_logs/gateway_timeout.log`
- Create: `sample_logs/normal_startup.log`
- Create: `.gitignore`

**Interfaces:**
- Produces: Working Go module with `google.golang.org/genai` and `github.com/google/uuid`, plus test fixture logs.

- [ ] **Step 1: Create .gitignore**

```gitignore
# Binaries
bin/
server

# Environment variables & secrets
.env
.env.local

# IDE
.vscode/
.idea/
.DS_Store

# Test artifacts
*.out
*.test
```

- [ ] **Step 2: Initialize go.mod and install dependencies**

Run:
```bash
go mod init sre-triage-agent
go get google.golang.org/genai@v1.63.0
go get github.com/google/uuid@v1.6.0
go mod tidy
```

- [ ] **Step 3: Create sample log fixtures**

`sample_logs/db_exhaustion.log`:
```text
2026-09-11T07:15:02Z INFO [api-gateway] Received POST /api/v1/orders from client 198.51.100.24 Authorization: Bearer secret-token-xyz-12345
2026-09-11T07:15:03Z ERROR [order-service] Failed to acquire connection from pool 'OrderDBPool': ConnectionPoolTimeoutException: Timeout waiting for idle connection after 30000ms. Active: 100/100, Idle: 0, Pending: 47
2026-09-11T07:15:04Z ERROR [order-service] HTTP 500 Internal Server Error: org.postgresql.util.PSQLException: ConnectionPoolTimeoutException: Pool exhausted
2026-09-11T07:15:05Z ERROR [api-gateway] Upstream error: order-service returned 500 Internal Server Error
2026-09-11T07:15:06Z ERROR [payment-service] DeadlockDetected: transaction 49201 waiting on lock held by transaction 49198 on table 'accounts'
2026-09-11T07:15:07Z FATAL [order-service] OutOfMemoryError / connection starvation: unable to process backlog, rejecting requests
```

`sample_logs/gateway_timeout.log`:
```text
2026-09-11T08:00:10Z INFO [frontend] GET /catalog/items duration=120ms status=200
2026-09-11T08:00:45Z WARN [api-gateway] Upstream request to inventory-service took 28400ms exceeding warning threshold (5000ms)
2026-09-11T08:01:12Z ERROR [api-gateway] HTTP 504 Gateway Timeout: upstream inventory-service failed to respond within 30000ms
2026-09-11T08:01:15Z WARN [inventory-service] High GC pause detected: 4820ms, JVM old-gen occupancy at 88%
2026-09-11T08:01:30Z WARN [inventory-service] Thread pool near capacity: 192/200 active threads
```

`sample_logs/normal_startup.log`:
```text
2026-09-11T06:00:00Z INFO [auth-service] Starting Auth Service v2.4.1 on port 8080
2026-09-11T06:00:01Z INFO [auth-service] Loaded 14 permission scopes from configuration
2026-09-11T06:00:02Z INFO [auth-service] Connected to Redis cache at 10.0.4.12:6379 (latency: 0.8ms)
2026-09-11T06:00:03Z WARN [auth-service] Configuration option 'legacy_tokens' is deprecated and will be removed in v3.0
2026-09-11T06:00:04Z INFO [auth-service] Health check listening on /healthz. Ready to accept connections.
```

- [ ] **Step 4: Verify module compiles**

Run: `go vet ./...`  
Expected: Clean exit code 0.

- [ ] **Step 5: Commit**

```bash
git add .gitignore go.mod go.sum sample_logs/
git commit -m "chore: setup go module and sample log fixtures"
```

---

### Task 2: SRE Tools Implementation (TDD)

**Files:**
- Create: `internal/tools/tools.go`
- Create: `internal/tools/read_log.go`
- Create: `internal/tools/parse_log.go`
- Create: `internal/tools/format_report.go`
- Test: `internal/tools/tools_test.go`

**Interfaces:**
- Produces:
  - `ReadLogFile(path string, maxLines int, offset int) (*ReadLogResult, error)`
  - `ParseLogSnippet(logText string) (*ParseLogResult, error)`
  - `FormatIncidentReport(params IncidentReportParams) (string, error)`
  - `MaskSensitiveData(input string) string`
  - `GetToolDeclarations() []*genai.Tool`
  - `DispatchTool(name string, argsJSON []byte) (any, error)`

- [ ] **Step 1: Write the failing tests for SRE tools**

`internal/tools/tools_test.go`:
```go
package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	res, err := ReadLogFile(logPath, 2, 1)
	if err != nil {
		t.Fatalf("ReadLogFile returned error: %v", err)
	}
	if res.TotalLines != 5 {
		t.Errorf("expected 5 total lines, got %d", res.TotalLines)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(res.Lines))
	}
	if res.Lines[0] != "line2" || res.Lines[1] != "line3" {
		t.Errorf("unexpected lines: %v", res.Lines)
	}
}

func TestParseLogSnippet(t *testing.T) {
	snippet := `
2026-09-11T07:15:02Z INFO [api-gateway] Request received
2026-09-11T07:15:03Z ERROR [order-service] ConnectionPoolTimeoutException: pool exhausted
2026-09-11T07:15:04Z ERROR [order-service] HTTP 500 Internal Server Error
2026-09-11T07:15:05Z WARN [cache] High memory warning
`
	res, err := ParseLogSnippet(snippet)
	if err != nil {
		t.Fatalf("ParseLogSnippet returned error: %v", err)
	}
	if res.LevelCounts["ERROR"] != 2 {
		t.Errorf("expected 2 errors, got %d", res.LevelCounts["ERROR"])
	}
	if res.LevelCounts["WARN"] != 1 {
		t.Errorf("expected 1 warn, got %d", res.LevelCounts["WARN"])
	}
	if len(res.HTTPStatusCodes) == 0 || res.HTTPStatusCodes[0] != "500" {
		t.Errorf("expected HTTP 500 code, got %v", res.HTTPStatusCodes)
	}
	if len(res.Exceptions) == 0 || !strings.Contains(res.Exceptions[0], "ConnectionPoolTimeoutException") {
		t.Errorf("expected ConnectionPoolTimeoutException, got %v", res.Exceptions)
	}
}

func TestMaskSensitiveData(t *testing.T) {
	input := "User token is Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.secret and password=supersecretpass with api_key=AIzaSyD-12345"
	masked := MaskSensitiveData(input)
	if strings.Contains(masked, "supersecretpass") {
		t.Errorf("password was not masked: %s", masked)
	}
	if strings.Contains(masked, "AIzaSyD-12345") {
		t.Errorf("api_key was not masked: %s", masked)
	}
	if !strings.Contains(masked, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output: %s", masked)
	}
}

func TestFormatIncidentReport(t *testing.T) {
	params := IncidentReportParams{
		IncidentID:       "INC-1001",
		Title:            "Database Connection Pool Saturation",
		Severity:         "CRITICAL",
		ImpactedServices: []string{"order-service", "api-gateway"},
		Timeline:         []string{"07:15:02 - Initial spike", "07:15:03 - Pool exhausted"},
		RootCause:        "Connection pool limit reached under high load without connection timeouts.",
		MitigationSteps:  []string{"Scale DB pool max connections to 200", "Enable connection leak detection"},
	}

	report, err := FormatIncidentReport(params)
	if err != nil {
		t.Fatalf("FormatIncidentReport returned error: %v", err)
	}
	if !strings.Contains(report, "# SRE Incident Report: INC-1001") {
		t.Errorf("missing header: %s", report)
	}
	if !strings.Contains(report, "**Severity**: `CRITICAL`") {
		t.Errorf("missing severity: %s", report)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tools/...`  
Expected: Compilation failure (functions not defined).

- [ ] **Step 3: Implement SRE tools**

`internal/tools/read_log.go`:
```go
package tools

import (
	"bufio"
	"fmt"
	"os"
)

type ReadLogArgs struct {
	FilePath string `json:"file_path"`
	MaxLines int    `json:"max_lines,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

type ReadLogResult struct {
	FilePath   string   `json:"file_path"`
	TotalLines int      `json:"total_lines"`
	Offset     int      `json:"offset"`
	LinesRead  int      `json:"lines_read"`
	Lines      []string `json:"lines"`
}

func ReadLogFile(path string, maxLines int, offset int) (*ReadLogResult, error) {
	if maxLines <= 0 {
		maxLines = 200
	}
	if maxLines > 1000 {
		maxLines = 1000
	}
	if offset < 0 {
		offset = 0
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file %q: %w", path, err)
	}

	total := len(allLines)
	if offset >= total {
		return &ReadLogResult{
			FilePath:   path,
			TotalLines: total,
			Offset:     offset,
			LinesRead:  0,
			Lines:      []string{},
		}, nil
	}

	end := offset + maxLines
	if end > total {
		end = total
	}

	sliced := allLines[offset:end]
	return &ReadLogResult{
		FilePath:   path,
		TotalLines: total,
		Offset:     offset,
		LinesRead:  len(sliced),
		Lines:      sliced,
	}, nil
}
```

`internal/tools/parse_log.go`:
```go
package tools

import (
	"regexp"
	"strings"
)

type ParseLogArgs struct {
	LogText string `json:"log_text"`
}

type ParseLogResult struct {
	TotalLines      int            `json:"total_lines"`
	LevelCounts     map[string]int `json:"level_counts"`
	HTTPStatusCodes []string       `json:"http_status_codes"`
	Exceptions      []string       `json:"exceptions"`
	ImpactedServices []string      `json:"impacted_services"`
	FirstTimestamp  string         `json:"first_timestamp,omitempty"`
	LastTimestamp   string         `json:"last_timestamp,omitempty"`
}

var (
	httpRegex      = regexp.MustCompile(`HTTP\s+([1-5][0-9]{2})`)
	exceptionRegex = regexp.MustCompile(`([A-Za-z0-9_]+(?:Exception|Error|Deadlock[A-Za-z0-9_]*))`)
	serviceRegex   = regexp.MustCompile(`\[([a-zA-Z0-9_-]+)\]`)
	timestampRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)`)
)

func ParseLogSnippet(logText string) (*ParseLogResult, error) {
	lines := strings.Split(strings.TrimSpace(logText), "\n")
	result := &ParseLogResult{
		TotalLines:  len(lines),
		LevelCounts: make(map[string]int),
	}

	httpCodesSet := make(map[string]struct{})
	exceptionsSet := make(map[string]struct{})
	servicesSet := make(map[string]struct{})

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		upper := strings.ToUpper(trimmed)
		switch {
		case strings.Contains(upper, "FATAL"):
			result.LevelCounts["FATAL"]++
		case strings.Contains(upper, "CRITICAL"):
			result.LevelCounts["CRITICAL"]++
		case strings.Contains(upper, "ERROR"):
			result.LevelCounts["ERROR"]++
		case strings.Contains(upper, "WARN"):
			result.LevelCounts["WARN"]++
		case strings.Contains(upper, "INFO"):
			result.LevelCounts["INFO"]++
		case strings.Contains(upper, "DEBUG"):
			result.LevelCounts["DEBUG"]++
		}

		if match := timestampRegex.FindString(trimmed); match != "" {
			if result.FirstTimestamp == "" {
				result.FirstTimestamp = match
			}
			result.LastTimestamp = match
		}

		for _, match := range httpRegex.FindAllStringSubmatch(trimmed, -1) {
			if len(match) > 1 {
				httpCodesSet[match[1]] = struct{}{}
			}
		}

		for _, match := range exceptionRegex.FindAllStringSubmatch(trimmed, -1) {
			if len(match) > 1 {
				exceptionsSet[match[1]] = struct{}{}
			}
		}

		for _, match := range serviceRegex.FindAllStringSubmatch(trimmed, -1) {
			if len(match) > 1 {
				servicesSet[match[1]] = struct{}{}
			}
		}
	}

	for code := range httpCodesSet {
		result.HTTPStatusCodes = append(result.HTTPStatusCodes, code)
	}
	for exc := range exceptionsSet {
		result.Exceptions = append(result.Exceptions, exc)
	}
	for svc := range servicesSet {
		result.ImpactedServices = append(result.ImpactedServices, svc)
	}

	return result, nil
}
```

`internal/tools/format_report.go`:
```go
package tools

import (
	"fmt"
	"regexp"
	"strings"
)

type IncidentReportParams struct {
	IncidentID          string   `json:"incident_id"`
	Title               string   `json:"title"`
	Severity            string   `json:"severity"` // CRITICAL, WARNING, INFO
	ImpactedServices    []string `json:"impacted_services"`
	Timeline            []string `json:"timeline"`
	RootCause           string   `json:"root_cause"`
	MitigationSteps     []string `json:"mitigation_steps"`
	PreventativeActions []string `json:"preventative_actions,omitempty"`
}

var (
	bearerTokenRegex = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-_.~+/]+=*`)
	passwordRegex    = regexp.MustCompile(`(?i)(password|passwd|secret)\s*[:=]\s*[^\s,;]+`)
	apiKeyRegex      = regexp.MustCompile(`(?i)(api[_-]?key)\s*[:=]\s*[A-Za-z0-9\-_]+`)
)

func MaskSensitiveData(input string) string {
	masked := bearerTokenRegex.ReplaceAllString(input, "Bearer [REDACTED]")
	masked = passwordRegex.ReplaceAllString(masked, "$1=[REDACTED]")
	masked = apiKeyRegex.ReplaceAllString(masked, "$1=[REDACTED]")
	return masked
}

func FormatIncidentReport(params IncidentReportParams) (string, error) {
	sev := strings.ToUpper(strings.TrimSpace(params.Severity))
	if sev != "CRITICAL" && sev != "WARNING" && sev != "INFO" {
		sev = "WARNING"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# SRE Incident Report: %s - %s\n\n", params.IncidentID, MaskSensitiveData(params.Title)))
	b.WriteString(fmt.Sprintf("- **Severity**: `%s`\n", sev))
	b.WriteString(fmt.Sprintf("- **Impacted Services**: `%s`\n\n", strings.Join(params.ImpactedServices, ", ")))

	b.WriteString("## Timeline\n")
	if len(params.Timeline) == 0 {
		b.WriteString("- No timeline events reported.\n")
	} else {
		for _, item := range params.Timeline {
			b.WriteString(fmt.Sprintf("- %s\n", MaskSensitiveData(item)))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Root Cause Analysis\n")
	b.WriteString(MaskSensitiveData(params.RootCause) + "\n\n")

	b.WriteString("## Immediate Mitigation Steps\n")
	for i, step := range params.MitigationSteps {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, MaskSensitiveData(step)))
	}
	b.WriteString("\n")

	if len(params.PreventativeActions) > 0 {
		b.WriteString("## Preventative Actions & Follow-ups\n")
		for i, action := range params.PreventativeActions {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, MaskSensitiveData(action)))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}
```

`internal/tools/tools.go`:
```go
package tools

import (
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

func GetToolDeclarations() []*genai.Tool {
	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "read_log_file",
					Description: "Reads bounded content from a local server log file with line offset and count support.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"file_path": {
								Type:        genai.TypeString,
								Description: "The relative or absolute path of the log file to inspect.",
							},
							"max_lines": {
								Type:        genai.TypeInteger,
								Description: "Maximum number of lines to read (default 200, max 1000).",
							},
							"offset": {
								Type:        genai.TypeInteger,
								Description: "Line offset from where to begin reading (default 0).",
							},
						},
						Required: []string{"file_path"},
					},
				},
				{
					Name:        "parse_log_snippet",
					Description: "Analyzes raw log lines to extract error counts, HTTP/gRPC codes, unique exceptions, and timestamps.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"log_text": {
								Type:        genai.TypeString,
								Description: "The raw log text to parse and categorize.",
							},
						},
						Required: []string{"log_text"},
					},
				},
				{
					Name:        "format_incident_report",
					Description: "Generates a standardized SRE postmortem incident report with severity classification and mitigation steps.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"incident_id": {
								Type:        genai.TypeString,
								Description: "Identifier for the incident, e.g. INC-1001.",
							},
							"title": {
								Type:        genai.TypeString,
								Description: "Clear summary title of the incident.",
							},
							"severity": {
								Type:        genai.TypeString,
								Description: "Incident severity: CRITICAL (outage/loss), WARNING (degraded/latency), INFO (routine/notice).",
							},
							"impacted_services": {
								Type: genai.TypeArray,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
								Description: "List of service names affected by this incident.",
							},
							"timeline": {
								Type: genai.TypeArray,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
								Description: "Chronological sequence of incident progression.",
							},
							"root_cause": {
								Type:        genai.TypeString,
								Description: "Technical explanation of why the failure occurred, backed by evidence from logs.",
							},
							"mitigation_steps": {
								Type: genai.TypeArray,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
								Description: "Immediate actions taken or required to resolve the issue.",
							},
							"preventative_actions": {
								Type: genai.TypeArray,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
								Description: "Long-term architecture or monitoring improvements to prevent recurrence.",
							},
						},
						Required: []string{"incident_id", "title", "severity", "impacted_services", "timeline", "root_cause", "mitigation_steps"},
					},
				},
			},
		},
	}
}

func DispatchTool(name string, argsMap map[string]any) (any, error) {
	data, err := json.Marshal(argsMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool arguments: %w", err)
	}

	switch name {
	case "read_log_file":
		var args ReadLogArgs
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for read_log_file: %w", err)
		}
		return ReadLogFile(args.FilePath, args.MaxLines, args.Offset)

	case "parse_log_snippet":
		var args ParseLogArgs
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for parse_log_snippet: %w", err)
		}
		return ParseLogSnippet(args.LogText)

	case "format_incident_report":
		var args IncidentReportParams
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for format_incident_report: %w", err)
		}
		report, err := FormatIncidentReport(args)
		if err != nil {
			return nil, err
		}
		return map[string]string{"report": report}, nil

	default:
		return nil, fmt.Errorf("unknown tool name: %q", name)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/tools/...`  
Expected: PASS with 4 tests passed.

- [ ] **Step 5: Commit**

```bash
git add internal/tools/
git commit -m "feat(tools): implement SRE log inspection and report formatting tools"
```

---

### Task 3: A2A Protocol & HTTP Transport (TDD)

**Files:**
- Create: `internal/a2a/types.go`
- Create: `internal/a2a/handler.go`
- Test: `internal/a2a/handler_test.go`

**Interfaces:**
- Consumes: Agent invoker function `type AgentInvoker func(ctx context.Context, message string, contextID string) (string, error)`
- Produces: `NewHandler(appName string, invoker AgentInvoker) http.Handler`

- [ ] **Step 1: Write failing tests for A2A endpoints**

`internal/a2a/handler_test.go`:
```go
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mockInvoker(_ context.Context, message string, contextID string) (string, error) {
	return "Echo response for: " + message + " [ctx: " + contextID + "]", nil
}

func TestAgentCardEndpoint(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	// Test root agent card
	req := httptest.NewRequest("GET", "/.well-known/agent-card.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("failed to decode agent card: %v", err)
	}
	if card.Name != "sre-triage-agent" {
		t.Errorf("expected name 'sre-triage-agent', got %q", card.Name)
	}
	if card.PreferredTransport != "JSONRPC" {
		t.Errorf("expected preferredTransport 'JSONRPC', got %q", card.PreferredTransport)
	}

	// Test prefixed agent card for Agent Runtime passthrough
	reqPrefixed := httptest.NewRequest("GET", "/a2a/sre-triage-agent/.well-known/agent-card.json", nil)
	wPrefixed := httptest.NewRecorder()
	handler.ServeHTTP(wPrefixed, reqPrefixed)
	if wPrefixed.Code != http.StatusOK {
		t.Fatalf("expected 200 for prefixed agent card, got %d", wPrefixed.Code)
	}
}

func TestA2AMessageSend(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	payload := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-1",
		Method:  "message/send",
		Params: MessageSendParams{
			Message: A2AMessage{
				Role:      "user",
				Parts:     []A2APart{{Kind: "text", Text: "Analyze incident"}},
				ContextID: "test-session-42",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/a2a/invoke", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if jsonResp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", jsonResp.Error)
	}
	if jsonResp.Result == nil {
		t.Fatalf("expected non-nil result")
	}
	if jsonResp.Result.ContextID != "test-session-42" {
		t.Errorf("expected contextId 'test-session-42', got %q", jsonResp.Result.ContextID)
	}
	if len(jsonResp.Result.Parts) == 0 || !strings.Contains(jsonResp.Result.Parts[0].Text, "Echo response") {
		t.Errorf("unexpected message response: %v", jsonResp.Result.Parts)
	}
}

func TestHealthCheck(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	req := httptest.NewRequest("GET", "/list-apps", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var apps []string
	if err := json.NewDecoder(w.Body).Decode(&apps); err != nil {
		t.Fatalf("failed to decode list-apps: %v", err)
	}
	if len(apps) == 0 || apps[0] != "sre-triage-agent" {
		t.Errorf("expected ['sre-triage-agent'], got %v", apps)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/a2a/...`  
Expected: Compilation failure.

- [ ] **Step 3: Implement A2A wire types and HTTP handler**

`internal/a2a/types.go`:
```go
package a2a

type AgentCard struct {
	ProtocolVersion    string           `json:"protocolVersion"`
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	URL                string           `json:"url"`
	PreferredTransport string           `json:"preferredTransport"`
	SupportedInterfaces []AgentInterface `json:"supportedInterfaces"`
	Capabilities       AgentCapabilities `json:"capabilities"`
	DefaultInputModes  []string         `json:"defaultInputModes"`
	DefaultOutputModes []string         `json:"defaultOutputModes"`
	Skills             []AgentSkill     `json:"skills"`
}

type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
}

type AgentCapabilities struct {
	Streaming bool `json:"streaming"`
}

type AgentSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type JSONRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      any               `json:"id"`
	Method  string            `json:"method"`
	Params  MessageSendParams `json:"params"`
}

type MessageSendParams struct {
	Message A2AMessage `json:"message"`
}

type A2AMessage struct {
	Role      string    `json:"role"`
	Parts     []A2APart `json:"parts"`
	TaskID    string    `json:"taskId,omitempty"`
	ContextID string    `json:"contextId,omitempty"`
}

type A2APart struct {
	Kind string `json:"kind,omitempty"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  *A2AMessage     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
```

`internal/a2a/handler.go`:
```go
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

type AgentInvoker func(ctx context.Context, message string, contextID string) (string, error)

type Handler struct {
	appName string
	invoker AgentInvoker
}

func NewHandler(appName string, invoker AgentInvoker) http.Handler {
	h := &Handler{
		appName: appName,
		invoker: invoker,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", h.handleAgentCard)
	mux.HandleFunc(fmt.Sprintf("/a2a/%s/.well-known/agent-card.json", appName), h.handleAgentCard)
	mux.HandleFunc("/a2a/invoke", h.handleInvoke)
	mux.HandleFunc(fmt.Sprintf("/a2a/%s/invoke", appName), h.handleInvoke)
	mux.HandleFunc("/list-apps", h.handleListApps)
	mux.HandleFunc("/", h.handleRoot)

	return mux
}

func (h *Handler) resolveBaseURL(r *http.Request) string {
	if envURL := os.Getenv("APP_URL"); envURL != "" {
		return strings.TrimSuffix(envURL, "/")
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func (h *Handler) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	baseURL := h.resolveBaseURL(r)
	invokeURL := fmt.Sprintf("%s/a2a/invoke", baseURL)

	card := AgentCard{
		ProtocolVersion:    "1.0",
		Name:               h.appName,
		Description:        "Autonomous SRE incident triage assistant for log parsing, anomaly classification, and postmortem generation.",
		URL:                invokeURL,
		PreferredTransport: "JSONRPC",
		SupportedInterfaces: []AgentInterface{
			{
				URL:             invokeURL,
				ProtocolBinding: "JSONRPC",
				ProtocolVersion: "1.0",
			},
		},
		Capabilities: AgentCapabilities{
			Streaming: true,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain", "text/markdown"},
		Skills: []AgentSkill{
			{
				ID:          "sre_incident_triage",
				Name:        "SRE Incident Triage",
				Description: "Parse server logs, classify incident severity, identify root causes, and format markdown incident reports.",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(card)
}

func (h *Handler) handleListApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode([]string{h.appName})
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.handleInvoke(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"agent":  h.appName,
	})
}

func (h *Handler) handleInvoke(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, nil, -32700, "Failed to read request body")
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, nil, -32700, "Parse error")
		return
	}

	if req.Method != "message/send" && req.Method != "tasks/create" {
		h.writeError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
		return
	}

	var promptText strings.Builder
	for _, part := range req.Params.Message.Parts {
		if part.Text != "" {
			if promptText.Len() > 0 {
				promptText.WriteString("\n")
			}
			promptText.WriteString(part.Text)
		}
	}

	contextID := req.Params.Message.ContextID
	if contextID == "" {
		contextID = uuid.New().String()
	}

	answer, err := h.invoker(r.Context(), promptText.String(), contextID)
	if err != nil {
		h.writeError(w, req.ID, -32000, fmt.Sprintf("Agent execution failed: %v", err))
		return
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: &A2AMessage{
			Role: "agent",
			Parts: []A2APart{
				{
					Kind: "text",
					Text: answer,
				},
			},
			ContextID: contextID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, id any, code int, msg string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: msg,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/a2a/...`  
Expected: PASS with 3 tests passed.

- [ ] **Step 5: Commit**

```bash
git add internal/a2a/
git commit -m "feat(a2a): implement A2A protocol card discovery and JSON-RPC invoke handler"
```

---

### Task 4: SRE Agent Core & Gemini Function Calling Loop (TDD)

**Files:**
- Create: `internal/agent/prompt.go`
- Create: `internal/agent/agent.go`
- Test: `internal/agent/agent_test.go`

**Interfaces:**
- Produces:
  - `NewAgent(ctx context.Context, cfg Config) (*Agent, error)`
  - `(a *Agent) Invoke(ctx context.Context, prompt string, contextID string) (string, error)`

- [ ] **Step 1: Write test for SRE agent prompt and dispatcher**

`internal/agent/agent_test.go`:
```go
package agent

import (
	"context"
	"strings"
	"testing"
)

func TestSystemPromptContent(t *testing.T) {
	prompt := GetSystemInstruction()
	if !strings.Contains(prompt, "SRE Incident Triage") {
		t.Errorf("expected SRE persona in instruction: %s", prompt)
	}
	if !strings.Contains(prompt, "CRITICAL") || !strings.Contains(prompt, "WARNING") || !strings.Contains(prompt, "INFO") {
		t.Errorf("missing severity guidelines: %s", prompt)
	}
	if !strings.Contains(prompt, "format_incident_report") {
		t.Errorf("missing mention of format_incident_report tool: %s", prompt)
	}
}

func TestMockAgentLoop(t *testing.T) {
	// Verifies agent configuration and validation without making live network calls
	cfg := Config{
		Model: "gemini-3.8-flash",
	}
	agent, err := NewAgentWithDispatcher(cfg, func(name string, args map[string]any) (any, error) {
		return "tool response", nil
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	if agent.ModelName != "gemini-3.8-flash" {
		t.Errorf("expected model 'gemini-3.8-flash', got %s", agent.ModelName)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/...`  
Expected: Compilation failure.

- [ ] **Step 3: Implement SRE agent and Gemini client**

`internal/agent/prompt.go`:
```go
package agent

func GetSystemInstruction() string {
	return `You are an expert autonomous Site Reliability Engineering (SRE) Incident Triage Agent.
Your objective is to ingest server and application logs, diagnose anomalies, determine root cause, classify severity, and produce a standardized SRE incident report.

### Core Workflow:
1. When asked to triage an incident or logs, inspect the target logs using the 'read_log_file' tool.
2. Use 'parse_log_snippet' to extract frequency of log levels, HTTP status codes, exceptions, and timestamps.
3. Classify the incident according to strict severity rules:
   - CRITICAL: Complete service outage, cascading failures, database connection pool exhaustion/deadlock, or data loss.
   - WARNING: Degraded performance, latency spikes (e.g. 504 Gateway Timeout), or resource saturation warnings without total outage.
   - INFO: Normal deployment, routine startup, non-blocking deprecations, healthy system status.
4. Call 'format_incident_report' with your findings to render the final markdown postmortem.
5. All secrets, bearer tokens, passwords, and API keys must remain masked or redacted ([REDACTED]).
6. Do not speculate on root causes without evidence found in the logs. If logs are insufficient, clearly state the missing telemetry in the report.`
}
```

`internal/agent/agent.go`:
```go
package agent

import (
	"context"
	"fmt"
	"log"
	"os"

	"sre-triage-agent/internal/tools"

	"google.golang.org/genai"
)

type Config struct {
	Model     string
	ProjectID string
	Location  string
	APIKey    string
}

type ToolDispatcher func(name string, args map[string]any) (any, error)

type Agent struct {
	client     *genai.Client
	ModelName  string
	dispatcher ToolDispatcher
}

func NewAgent(ctx context.Context, cfg Config) (*Agent, error) {
	if cfg.Model == "" {
		cfg.Model = os.Getenv("GEMINI_MODEL")
		if cfg.Model == "" {
			cfg.Model = "gemini-3.8-flash"
		}
	}

	clientCfg := &genai.ClientConfig{}

	// Check if Vertex AI should be used
	project := cfg.ProjectID
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	location := cfg.Location
	if location == "" {
		location = os.Getenv("GOOGLE_CLOUD_LOCATION")
		if location == "" {
			location = "global"
		}
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	if project != "" || os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") == "true" {
		clientCfg.Backend = genai.BackendVertexAI
		clientCfg.Project = project
		clientCfg.Location = location
		log.Printf("Agent using Vertex AI Backend (project=%s, location=%s)", project, location)
	} else if apiKey != "" {
		clientCfg.Backend = genai.BackendGeminiAPI
		clientCfg.APIKey = apiKey
		log.Println("Agent using Gemini API Backend via API Key")
	} else {
		// Default to Vertex AI with ADC
		clientCfg.Backend = genai.BackendVertexAI
		clientCfg.Location = location
		log.Println("Agent defaulting to Vertex AI Backend with Application Default Credentials")
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Agent{
		client:     client,
		ModelName:  cfg.Model,
		dispatcher: tools.DispatchTool,
	}, nil
}

func NewAgentWithDispatcher(cfg Config, dispatcher ToolDispatcher) (*Agent, error) {
	if cfg.Model == "" {
		cfg.Model = "gemini-3.8-flash"
	}
	return &Agent{
		ModelName:  cfg.Model,
		dispatcher: dispatcher,
	}, nil
}

func (a *Agent) Invoke(ctx context.Context, userPrompt string, contextID string) (string, error) {
	if a.client == nil {
		return "Agent client is uninitialized. Ensure credentials (ADC or GEMINI_API_KEY) are configured.", nil
	}

	systemContent := &genai.Content{
		Role: "system",
		Parts: []*genai.Part{
			{Text: GetSystemInstruction()},
		},
	}

	history := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: userPrompt},
			},
		},
	}

	toolDefs := tools.GetToolDeclarations()
	maxTurns := 8

	for turn := 0; turn < maxTurns; turn++ {
		genCfg := &genai.GenerateContentConfig{
			SystemInstruction: systemContent,
			Tools:             toolDefs,
		}

		resp, err := a.client.Models.GenerateContent(ctx, a.ModelName, history, genCfg)
		if err != nil {
			return "", fmt.Errorf("gemini generate content error: %w", err)
		}

		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			return "No response generated by model.", nil
		}

		candidate := resp.Candidates[0]
		history = append(history, candidate.Content)

		var functionCalls []*genai.FunctionCall
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				functionCalls = append(functionCalls, part.FunctionCall)
			}
		}

		// If no tools were called, return the text content
		if len(functionCalls) == 0 {
			var textOutput string
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					textOutput += part.Text
				}
			}
			return textOutput, nil
		}

		// Execute function calls
		var responseParts []*genai.Part
		for _, fc := range functionCalls {
			result, err := a.dispatcher(fc.Name, fc.Args)
			if err != nil {
				result = map[string]string{"error": err.Error()}
			}
			resultMap, ok := result.(map[string]any)
			if !ok {
				// Convert to generic map
				resultMap = map[string]any{"output": result}
			}

			responseParts = append(responseParts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     fc.Name,
					Response: resultMap,
				},
			})
		}

		history = append(history, &genai.Content{
			Role:  "tool",
			Parts: responseParts,
		})
	}

	return "Reached maximum triage loop iterations.", nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/agent/...`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/
git commit -m "feat(agent): implement SRE agent function-calling loop with Gemini 3.8 Flash"
```

---

### Task 5: Server Entrypoint & Integration Tests

**Files:**
- Create: `cmd/server/main.go`
- Test: `cmd/server/main_test.go`

**Interfaces:**
- Produces: Executable server supporting `--port` and environment variables `PORT`, `APP_URL`, `GEMINI_MODEL`.

- [ ] **Step 1: Write integration test for server initialization**

`cmd/server/main_test.go`:
```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"sre-triage-agent/internal/a2a"
)

func TestServerRouterHealth(t *testing.T) {
	mockInvoker := func(ctx context.Context, prompt string, contextID string) (string, error) {
		return "OK", nil
	}

	handler := a2a.NewHandler("sre-triage-agent", mockInvoker)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/.well-known/agent-card.json")
	if err != nil {
		t.Fatalf("failed to fetch agent card: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Implement server main entrypoint**

`cmd/server/main.go`:
```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sre-triage-agent/internal/a2a"
	"sre-triage-agent/internal/agent"
)

func main() {
	portFlag := flag.String("port", "", "Port to listen on (defaults to $PORT or 8080)")
	modelFlag := flag.String("model", "", "Gemini model (defaults to $GEMINI_MODEL or gemini-3.8-flash)")
	flag.Parse()

	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
	}

	model := *modelFlag
	if model == "" {
		model = os.Getenv("GEMINI_MODEL")
		if model == "" {
			model = "gemini-3.8-flash"
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sreAgent, err := agent.NewAgent(ctx, agent.Config{
		Model: model,
	})
	if err != nil {
		log.Printf("Warning: Live GenAI client initialization failed (%v). Starting server in fallback mode.", err)
	}

	invoker := func(ctx context.Context, message string, contextID string) (string, error) {
		if sreAgent == nil {
			return "Agent credentials not configured. Set GEMINI_API_KEY or GOOGLE_CLOUD_PROJECT with ADC.", nil
		}
		return sreAgent.Invoke(ctx, message, contextID)
	}

	handler := a2a.NewHandler("sre-triage-agent", invoker)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	go func() {
		log.Printf("SRE Triage Agent listening on port %s", port)
		log.Printf("Agent Card: http://localhost:%s/.well-known/agent-card.json", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server exited cleanly.")
}
```

- [ ] **Step 3: Run tests and build binary**

Run:
```bash
go test -v ./cmd/server/...
go build -o bin/server ./cmd/server
```
Expected: Clean build and test pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/server/
git commit -m "feat(server): add server main entrypoint and integration tests"
```

---

### Task 6: Containerization & Agent CLI Manifest

**Files:**
- Create: `Dockerfile`
- Create: `agents-cli-manifest.yaml`
- Create: `.gcloudignore`

**Interfaces:**
- Produces: Valid Docker container buildable for Agent Runtime, and `agents-cli-manifest.yaml` recognized by `agents-cli`.

- [ ] **Step 1: Create Dockerfile**

`Dockerfile`:
```dockerfile
# Build stage
FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/sre-agent ./cmd/server

# Production runtime stage
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /bin/sre-agent /app/sre-agent
COPY sample_logs/ /app/sample_logs/

USER nonroot:nonroot
EXPOSE 8080
ENV PORT=8080

ENTRYPOINT ["/app/sre-agent"]
```

- [ ] **Step 2: Create agents-cli-manifest.yaml**

`agents-cli-manifest.yaml`:
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

- [ ] **Step 3: Create .gcloudignore**

`.gcloudignore`:
```text
.git/
.gitignore
bin/
docs/
*.md
```

- [ ] **Step 4: Commit**

```bash
git add Dockerfile agents-cli-manifest.yaml .gcloudignore
git commit -m "chore(deploy): add multi-stage Dockerfile and agents-cli manifest"
```

---

### Task 7: End-to-End Verification with `agents-cli run --mode a2a`

**Files:**
- Modify/Verify: Local running server and `agents-cli` execution

- [ ] **Step 1: Run all unit and package tests**

Run: `go test -v ./...`  
Expected: All package tests pass.

- [ ] **Step 2: Start server locally on port 8080 in background**

Run:
```bash
./bin/server --port 8080 &
SERVER_PID=$!
sleep 2
```

- [ ] **Step 3: Verify agent card discovery endpoint**

Run:
```bash
curl -s http://localhost:8080/.well-known/agent-card.json
```
Expected: HTTP 200 with valid JSON containing `"name":"sre-triage-agent"` and `"preferredTransport":"JSONRPC"`.

- [ ] **Step 4: Query the running agent using Agent CLI**

Run:
```bash
agents-cli run --url http://localhost:8080 --mode a2a "What are your SRE triage capabilities?"
```
Expected: Agent answers describing its log reading, anomaly parsing, and incident postmortem formatting capabilities.

- [ ] **Step 5: Run triage against sample log**

Run:
```bash
agents-cli run --url http://localhost:8080 --mode a2a "Triage the incident in sample_logs/db_exhaustion.log"
```
Expected: Output includes formatted SRE Incident Report with `CRITICAL` severity and mitigation steps.

- [ ] **Step 6: Stop local server**

Run: `kill $SERVER_PID`

- [ ] **Step 7: Final commit and status check**

```bash
git status
git commit -am "chore: complete verification and testing"
```
