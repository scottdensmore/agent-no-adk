package tools

import (
	"regexp"
	"sort"
	"strings"
)

type ParseLogArgs struct {
	LogText string `json:"log_text"`
}

type ParseLogResult struct {
	TotalLines       int            `json:"total_lines"`
	LevelCounts      map[string]int `json:"level_counts"`
	HTTPStatusCodes []string       `json:"http_status_codes"`
	Exceptions       []string       `json:"exceptions"`
	ImpactedServices []string       `json:"impacted_services"`
	FirstTimestamp   string         `json:"first_timestamp,omitempty"`
	LastTimestamp    string         `json:"last_timestamp,omitempty"`
}

var (
	httpRegex      = regexp.MustCompile(`HTTP\s+([1-5][0-9]{2})`)
	exceptionRegex = regexp.MustCompile(`([A-Za-z0-9_]+(?:Exception|Error|Deadlock[A-Za-z0-9_]*))`)
	serviceRegex   = regexp.MustCompile(`\[([a-zA-Z0-9_-]+)\]`)
	timestampRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)`)
)

func ParseLogSnippet(logText string) (*ParseLogResult, error) {
	trimmedText := strings.TrimSpace(logText)
	if trimmedText == "" {
		return &ParseLogResult{
			TotalLines:       0,
			LevelCounts:      make(map[string]int),
			HTTPStatusCodes: []string{},
			Exceptions:       []string{},
			ImpactedServices: []string{},
		}, nil
	}

	lines := strings.Split(trimmedText, "\n")
	result := &ParseLogResult{
		TotalLines:       len(lines),
		LevelCounts:      make(map[string]int),
		HTTPStatusCodes: []string{},
		Exceptions:       []string{},
		ImpactedServices: []string{},
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
	sort.Strings(result.HTTPStatusCodes)

	for exc := range exceptionsSet {
		result.Exceptions = append(result.Exceptions, exc)
	}
	sort.Strings(result.Exceptions)

	for svc := range servicesSet {
		result.ImpactedServices = append(result.ImpactedServices, svc)
	}
	sort.Strings(result.ImpactedServices)

	return result, nil
}
