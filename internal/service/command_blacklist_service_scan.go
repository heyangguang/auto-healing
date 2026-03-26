package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

const maxScanFileSize = 10 * 1024 * 1024

// ScanWorkspace 扫描工作空间，检测高危指令
func (s *CommandBlacklistService) ScanWorkspace(ctx context.Context, workDir string) ([]model.CommandBlacklistViolation, error) {
	compiled, err := s.compiledActiveRules(ctx)
	if err != nil || len(compiled) == 0 {
		return nil, err
	}

	var violations []model.CommandBlacklistViolation
	err = filepath.Walk(workDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if skipScanEntry(path, info) {
			if info.IsDir() && info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := filepath.Rel(workDir, path)
		violations = append(violations, s.scanFile(path, relPath, compiled)...)
		return nil
	})
	if err != nil {
		return violations, fmt.Errorf("扫描工作空间失败: %w", err)
	}
	return violations, nil
}

func (s *CommandBlacklistService) compiledActiveRules(ctx context.Context) ([]compiledRule, error) {
	rules, err := s.repo.GetActiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取黑名单规则失败: %w", err)
	}
	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		cr := compiledRule{rule: rule}
		if rule.MatchType == "regex" {
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				logger.Exec("SECURITY").Warn("黑名单规则正则编译失败，跳过: %s (%v)", rule.Name, err)
				continue
			}
			cr.regex = re
		}
		compiled = append(compiled, cr)
	}
	return compiled, nil
}

func skipScanEntry(path string, info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	if info.Size() > maxScanFileSize {
		return true
	}
	return blacklistedBinaryExtensions[strings.ToLower(filepath.Ext(path))] || !isTextFile(path)
}

var blacklistedBinaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".bmp": true, ".svg": true, ".webp": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true, ".rar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true, ".dat": true, ".db": true, ".sqlite": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".pdf": true, ".doc": true, ".docx": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mkv": true, ".pyc": true, ".class": true, ".o": true,
}

type compiledRule struct {
	rule  model.CommandBlacklist
	regex *regexp.Regexp
}

func (s *CommandBlacklistService) scanFile(filePath, relPath string, rules []compiledRule) []model.CommandBlacklistViolation {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var violations []model.CommandBlacklistViolation
	scanner := bufio.NewScanner(file)
	for lineNum := 1; scanner.Scan(); lineNum++ {
		line := scanner.Text()
		for _, rule := range rules {
			if !matchLine(line, rule.rule.Pattern, rule.rule.MatchType, rule.regex) {
				continue
			}
			violations = append(violations, model.CommandBlacklistViolation{
				RuleID:   rule.rule.ID,
				File:     relPath,
				Line:     lineNum,
				Content:  strings.TrimSpace(truncateLine(line, 200)),
				RuleName: rule.rule.Name,
				Pattern:  rule.rule.Pattern,
				Severity: rule.rule.Severity,
			})
		}
	}
	return violations
}

func isTextFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return false
		}
	}
	return true
}
