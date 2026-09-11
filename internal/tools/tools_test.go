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

func TestReadLogFile_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_edge.log")
	content := "line1\nline2\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	// 1. Nonexistent file
	_, err := ReadLogFile(filepath.Join(tmpDir, "does_not_exist.log"), 10, 0)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}

	// 2. Offset >= total lines
	res, err := ReadLogFile(logPath, 10, 5)
	if err != nil {
		t.Fatalf("ReadLogFile returned error: %v", err)
	}
	if res.LinesRead != 0 || len(res.Lines) != 0 {
		t.Errorf("expected 0 lines read, got %d", res.LinesRead)
	}

	// 3. Default maxLines and negative offset clamp
	res, err = ReadLogFile(logPath, 0, -5)
	if err != nil {
		t.Fatalf("ReadLogFile returned error: %v", err)
	}
	if res.Offset != 0 {
		t.Errorf("expected offset clamped to 0, got %d", res.Offset)
	}
	if len(res.Lines) != 2 {
		t.Errorf("expected 2 lines read with default maxLines, got %d", len(res.Lines))
	}

	// 4. maxLines clamping > 1000
	res, err = ReadLogFile(logPath, 2000, 0)
	if err != nil {
		t.Fatalf("ReadLogFile returned error: %v", err)
	}
	if len(res.Lines) != 2 {
		t.Errorf("expected 2 lines read, got %d", len(res.Lines))
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
	if res.FirstTimestamp != "2026-09-11T07:15:02Z" {
		t.Errorf("expected first timestamp 2026-09-11T07:15:02Z, got %s", res.FirstTimestamp)
	}
	if res.LastTimestamp != "2026-09-11T07:15:05Z" {
		t.Errorf("expected last timestamp 2026-09-11T07:15:05Z, got %s", res.LastTimestamp)
	}
}

func TestParseLogSnippet_Empty(t *testing.T) {
	res, err := ParseLogSnippet("   \n   \n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalLines != 0 {
		t.Errorf("expected 0 total lines, got %d", res.TotalLines)
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
		IncidentID:          "INC-1001",
		Title:               "Database Connection Pool Saturation",
		Severity:            "CRITICAL",
		ImpactedServices:    []string{"order-service", "api-gateway"},
		Timeline:            []string{"07:15:02 - Initial spike", "07:15:03 - Pool exhausted"},
		RootCause:           "Connection pool limit reached under high load without connection timeouts.",
		MitigationSteps:     []string{"Scale DB pool max connections to 200", "Enable connection leak detection"},
		PreventativeActions: []string{"Add automated alert on 80% pool utilization"},
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
	if !strings.Contains(report, "## Preventative Actions & Follow-ups") {
		t.Errorf("missing preventative actions: %s", report)
	}
}

func TestFormatIncidentReport_DefaultSeverityAndEmptyTimeline(t *testing.T) {
	params := IncidentReportParams{
		IncidentID:      "INC-1002",
		Title:           "Minor hiccup",
		Severity:        "UNKNOWN_SEV",
		MitigationSteps: []string{"Restart service"},
	}
	report, err := FormatIncidentReport(params)
	if err != nil {
		t.Fatalf("FormatIncidentReport returned error: %v", err)
	}
	if !strings.Contains(report, "**Severity**: `WARNING`") {
		t.Errorf("expected default severity WARNING, got: %s", report)
	}
	if !strings.Contains(report, "No timeline events reported.") {
		t.Errorf("expected empty timeline message, got: %s", report)
	}
}

func TestGetToolDeclarations(t *testing.T) {
	tools := GetToolDeclarations()
	if len(tools) != 1 {
		t.Fatalf("expected 1 Tool grouping, got %d", len(tools))
	}
	fnDecls := tools[0].FunctionDeclarations
	if len(fnDecls) != 3 {
		t.Fatalf("expected 3 function declarations, got %d", len(fnDecls))
	}
	names := map[string]bool{}
	for _, fn := range fnDecls {
		names[fn.Name] = true
	}
	expected := []string{"read_log_file", "parse_log_snippet", "format_incident_report"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool declaration %q not found", name)
		}
	}
}

func TestDispatchTool(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "dispatch_test.log")
	if err := os.WriteFile(logPath, []byte("lineA\nlineB\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. read_log_file dispatch
	resRead, err := DispatchTool("read_log_file", map[string]any{
		"file_path": logPath,
		"max_lines": 1,
		"offset":    0,
	})
	if err != nil {
		t.Fatalf("DispatchTool read_log_file failed: %v", err)
	}
	readResult, ok := resRead.(*ReadLogResult)
	if !ok {
		t.Fatalf("expected *ReadLogResult, got %T", resRead)
	}
	if len(readResult.Lines) != 1 || readResult.Lines[0] != "lineA" {
		t.Errorf("unexpected lines read: %v", readResult.Lines)
	}

	// 2. parse_log_snippet dispatch
	resParse, err := DispatchTool("parse_log_snippet", map[string]any{
		"log_text": "2026-09-11T07:15:02Z ERROR [test-svc] HTTP 503 Service Unavailable",
	})
	if err != nil {
		t.Fatalf("DispatchTool parse_log_snippet failed: %v", err)
	}
	parseResult, ok := resParse.(*ParseLogResult)
	if !ok {
		t.Fatalf("expected *ParseLogResult, got %T", resParse)
	}
	if parseResult.LevelCounts["ERROR"] != 1 {
		t.Errorf("expected 1 ERROR, got %d", parseResult.LevelCounts["ERROR"])
	}

	// 3. format_incident_report dispatch
	resFormat, err := DispatchTool("format_incident_report", map[string]any{
		"incident_id":       "INC-2000",
		"title":             "Test Outage",
		"severity":          "WARNING",
		"impacted_services": []string{"svc-a"},
		"timeline":          []string{"timeline 1"},
		"root_cause":        "test cause",
		"mitigation_steps":  []string{"step 1"},
	})
	if err != nil {
		t.Fatalf("DispatchTool format_incident_report failed: %v", err)
	}
	formatResult, ok := resFormat.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", resFormat)
	}
	if !strings.Contains(formatResult["report"], "INC-2000") {
		t.Errorf("expected incident ID in report, got %s", formatResult["report"])
	}

	// 4. unknown tool
	_, err = DispatchTool("unknown_tool", map[string]any{})
	if err == nil {
		t.Error("expected error for unknown tool, got nil")
	}

	// 5. invalid arguments error handling
	_, err = DispatchTool("read_log_file", map[string]any{
		"file_path": 12345, // invalid type
	})
	if err == nil {
		t.Error("expected error for invalid read_log_file args, got nil")
	}

	_, err = DispatchTool("parse_log_snippet", map[string]any{
		"log_text": 12345, // invalid type
	})
	if err == nil {
		t.Error("expected error for invalid parse_log_snippet args, got nil")
	}

	_, err = DispatchTool("format_incident_report", map[string]any{
		"incident_id": 12345, // invalid type
	})
	if err == nil {
		t.Error("expected error for invalid format_incident_report args, got nil")
	}
}
