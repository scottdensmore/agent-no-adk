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
