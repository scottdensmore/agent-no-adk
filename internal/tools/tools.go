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
		if args.FilePath == "" {
			for _, k := range []string{"path", "filePath", "filepath", "file", "filename"} {
				if s, ok := argsMap[k].(string); ok && s != "" {
					args.FilePath = s
					break
				}
			}
		}
		return ReadLogFile(args.FilePath, args.MaxLines, args.Offset)

	case "parse_log_snippet":
		var args ParseLogArgs
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for parse_log_snippet: %w", err)
		}
		if args.LogText == "" {
			for _, k := range []string{"raw_log", "rawLog", "text", "log", "snippet"} {
				if s, ok := argsMap[k].(string); ok && s != "" {
					args.LogText = s
					break
				}
			}
		}
		return ParseLogSnippet(args.LogText)

	case "format_incident_report":
		var args IncidentReportParams
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for format_incident_report: %w", err)
		}
		if args.IncidentID == "" {
			args.IncidentID = "INC-1001"
		}
		if len(args.MitigationSteps) == 0 {
			if items, ok := argsMap["action_items"].([]any); ok {
				for _, it := range items {
					if s, ok := it.(string); ok {
						args.MitigationSteps = append(args.MitigationSteps, s)
					}
				}
			}
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
