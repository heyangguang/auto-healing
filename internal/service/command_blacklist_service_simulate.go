package service

import (
	"fmt"
	"regexp"
	"strings"
)

// Simulate 仿真测试 — 使用与 ScanWorkspace/scanFile 完全一致的匹配引擎
func (s *CommandBlacklistService) Simulate(req *SimulateRequest) ([]SimulateResult, error) {
	if req.MatchType != "contains" && req.MatchType != "regex" && req.MatchType != "exact" {
		return nil, fmt.Errorf("无效的匹配类型: %s", req.MatchType)
	}
	var re *regexp.Regexp
	if req.MatchType == "regex" {
		var err error
		re, err = regexp.Compile(req.Pattern)
		if err != nil {
			return nil, fmt.Errorf("无效的正则表达式: %w", err)
		}
	}
	if len(req.Files) > 0 {
		return simulateFiles(req, re), nil
	}
	return simulateContent(req, re), nil
}

func simulateFiles(req *SimulateRequest, re *regexp.Regexp) []SimulateResult {
	var results []SimulateResult
	globalLine := 0
	for _, file := range req.Files {
		for _, line := range strings.Split(file.Content, "\n") {
			globalLine++
			results = append(results, SimulateResult{
				Line:    globalLine,
				Content: truncateLine(line, 200),
				Matched: matchLine(line, req.Pattern, req.MatchType, re),
				File:    file.Path,
			})
		}
	}
	return results
}

func simulateContent(req *SimulateRequest, re *regexp.Regexp) []SimulateResult {
	lines := strings.Split(req.Content, "\n")
	results := make([]SimulateResult, 0, len(lines))
	for i, line := range lines {
		results = append(results, SimulateResult{
			Line:    i + 1,
			Content: truncateLine(line, 200),
			Matched: matchLine(line, req.Pattern, req.MatchType, re),
		})
	}
	return results
}

func matchLine(line, pattern, matchType string, re *regexp.Regexp) bool {
	switch matchType {
	case "contains":
		return strings.Contains(line, pattern)
	case "exact":
		return strings.TrimSpace(line) == pattern
	case "regex":
		return re != nil && re.MatchString(line)
	default:
		return false
	}
}

func truncateLine(line string, maxLen int) string {
	if len(line) > maxLen {
		return line[:maxLen] + "..."
	}
	return line
}
