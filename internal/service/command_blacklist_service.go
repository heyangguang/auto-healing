package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

// CommandBlacklistService 高危指令黑名单服务
type CommandBlacklistService struct {
	repo *repository.CommandBlacklistRepository
}

// NewCommandBlacklistService 创建服务
func NewCommandBlacklistService() *CommandBlacklistService {
	return &CommandBlacklistService{
		repo: repository.NewCommandBlacklistRepository(),
	}
}

// Create 创建规则
func (s *CommandBlacklistService) Create(ctx context.Context, rule *model.CommandBlacklist) error {
	rule.ID = uuid.New()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	// 注入租户 ID
	tenantID := repository.TenantIDFromContext(ctx)
	if tenantID != uuid.Nil {
		rule.TenantID = &tenantID
	}

	// 验证 match_type
	if rule.MatchType == "" {
		rule.MatchType = "contains"
	}
	if rule.MatchType != "contains" && rule.MatchType != "regex" && rule.MatchType != "exact" {
		return fmt.Errorf("无效的匹配类型: %s, 支持 contains/regex/exact", rule.MatchType)
	}

	// 如果是正则表达式，验证语法
	if rule.MatchType == "regex" {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("无效的正则表达式: %w", err)
		}
	}

	// 验证 severity
	if rule.Severity == "" {
		rule.Severity = "critical"
	}
	if rule.Severity != "critical" && rule.Severity != "high" && rule.Severity != "medium" {
		return fmt.Errorf("无效的严重级别: %s, 支持 critical/high/medium", rule.Severity)
	}

	return s.repo.Create(ctx, rule)
}

// GetByID 获取规则
func (s *CommandBlacklistService) GetByID(ctx context.Context, id uuid.UUID) (*model.CommandBlacklist, error) {
	return s.repo.GetByID(ctx, id)
}

// List 列表查询
func (s *CommandBlacklistService) List(ctx context.Context, opts *repository.CommandBlacklistListOptions) ([]model.CommandBlacklist, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 20
	}
	return s.repo.List(ctx, opts)
}

// Update 更新规则
func (s *CommandBlacklistService) Update(ctx context.Context, id uuid.UUID, input *model.CommandBlacklist) (*model.CommandBlacklist, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if input.Name != "" {
		rule.Name = input.Name
	}
	if input.Pattern != "" {
		rule.Pattern = input.Pattern
	}
	if input.MatchType != "" {
		if input.MatchType != "contains" && input.MatchType != "regex" && input.MatchType != "exact" {
			return nil, fmt.Errorf("无效的匹配类型: %s", input.MatchType)
		}
		rule.MatchType = input.MatchType
	}
	if input.Severity != "" {
		if input.Severity != "critical" && input.Severity != "high" && input.Severity != "medium" {
			return nil, fmt.Errorf("无效的严重级别: %s", input.Severity)
		}
		rule.Severity = input.Severity
	}

	// 如果是正则表达式，验证语法
	if rule.MatchType == "regex" {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return nil, fmt.Errorf("无效的正则表达式: %w", err)
		}
	}

	rule.Category = input.Category
	rule.Description = input.Description
	rule.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// Delete 删除规则
func (s *CommandBlacklistService) Delete(ctx context.Context, id uuid.UUID) error {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if rule.IsSystem {
		return fmt.Errorf("系统内置规则不可删除")
	}
	return s.repo.Delete(ctx, id)
}

// ToggleActive 启用/禁用
func (s *CommandBlacklistService) ToggleActive(ctx context.Context, id uuid.UUID) (*model.CommandBlacklist, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rule.IsActive = !rule.IsActive
	rule.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// BatchToggle 批量启用/禁用
func (s *CommandBlacklistService) BatchToggle(ctx context.Context, ids []uuid.UUID, isActive bool) (int64, error) {
	return s.repo.BatchToggle(ctx, ids, isActive)
}

// ScanWorkspace 扫描工作空间，检测高危指令
// 扫描所有文本文件（排除二进制文件和 .git 目录）
func (s *CommandBlacklistService) ScanWorkspace(ctx context.Context, workDir string) ([]model.CommandBlacklistViolation, error) {
	// 获取当前租户所有启用的规则
	rules, err := s.repo.GetActiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取黑名单规则失败: %w", err)
	}

	if len(rules) == 0 {
		return nil, nil // 没有规则，直接通过
	}

	// 编译正则表达式（缓存）
	var compiled []compiledRule
	for _, rule := range rules {
		cr := compiledRule{rule: rule}
		switch rule.MatchType {
		case "regex":
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				logger.Exec("SECURITY").Warn("黑名单规则正则编译失败，跳过: %s (%v)", rule.Name, err)
				continue
			}
			cr.regex = re
		}
		compiled = append(compiled, cr)
	}

	var violations []model.CommandBlacklistViolation

	// 遍历工作空间所有文件
	err = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		// 跳过目录
		if info.IsDir() {
			// 跳过 .git 目录
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过大文件（> 10MB，可能是二进制文件）
		if info.Size() > 10*1024*1024 {
			return nil
		}

		// 跳过已知的二进制文件扩展名
		ext := strings.ToLower(filepath.Ext(path))
		binaryExts := map[string]bool{
			".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
			".ico": true, ".bmp": true, ".svg": true, ".webp": true,
			".zip": true, ".tar": true, ".gz": true, ".bz2": true,
			".xz": true, ".7z": true, ".rar": true,
			".exe": true, ".dll": true, ".so": true, ".dylib": true,
			".bin": true, ".dat": true, ".db": true, ".sqlite": true,
			".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
			".pdf": true, ".doc": true, ".docx": true,
			".mp3": true, ".mp4": true, ".avi": true, ".mkv": true,
			".pyc": true, ".class": true, ".o": true,
		}
		if binaryExts[ext] {
			return nil
		}

		// 检查文件是否为文本文件（读取前512字节检测）
		if !isTextFile(path) {
			return nil
		}

		// 计算相对路径
		relPath, _ := filepath.Rel(workDir, path)

		// 扫描文件内容
		fileViolations := s.scanFile(path, relPath, compiled)
		violations = append(violations, fileViolations...)

		return nil
	})

	if err != nil {
		return violations, fmt.Errorf("扫描工作空间失败: %w", err)
	}

	return violations, nil
}

// compiledRule 编译后的规则
type compiledRule struct {
	rule  model.CommandBlacklist
	regex *regexp.Regexp
}

// scanFile 扫描单个文件
func (s *CommandBlacklistService) scanFile(filePath, relPath string, rules []compiledRule) []model.CommandBlacklistViolation {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var violations []model.CommandBlacklistViolation
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, cr := range rules {
			matched := false

			switch cr.rule.MatchType {
			case "contains":
				matched = strings.Contains(line, cr.rule.Pattern)
			case "exact":
				matched = strings.TrimSpace(line) == cr.rule.Pattern
			case "regex":
				if cr.regex != nil {
					matched = cr.regex.MatchString(line)
				}
			}

			if matched {
				// 截断过长的行
				content := line
				if len(content) > 200 {
					content = content[:200] + "..."
				}

				violations = append(violations, model.CommandBlacklistViolation{
					File:     relPath,
					Line:     lineNum,
					Content:  strings.TrimSpace(content),
					RuleName: cr.rule.Name,
					Pattern:  cr.rule.Pattern,
					Severity: cr.rule.Severity,
				})
			}
		}
	}

	return violations
}

// isTextFile 检查文件是否为文本文件
func isTextFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// 读取前512字节
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil || n == 0 {
		return false
	}

	// 检查是否包含 null 字节（二进制文件特征）
	for _, b := range buf[:n] {
		if b == 0 {
			return false
		}
	}
	return true
}

// GetActiveRules 获取所有启用的规则（用于外部调用）
func (s *CommandBlacklistService) GetActiveRules(ctx context.Context) ([]model.CommandBlacklist, error) {
	return s.repo.GetActiveRules(ctx)
}

// SimulateResult 仿真测试单行结果
type SimulateResult struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
	Matched bool   `json:"matched"`
	File    string `json:"file,omitempty"`
}

// SimulateRequest 仿真测试请求
type SimulateRequest struct {
	Pattern   string            `json:"pattern" binding:"required"`
	MatchType string            `json:"match_type" binding:"required"`
	Files     []SimulateFileReq `json:"files"`   // 模板模式：按文件传入
	Content   string            `json:"content"` // 手动模式：纯文本
}

// SimulateFileReq 单个文件
type SimulateFileReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Simulate 仿真测试 — 使用与 ScanWorkspace/scanFile 完全一致的匹配引擎
func (s *CommandBlacklistService) Simulate(req *SimulateRequest) ([]SimulateResult, error) {
	// 验证 match_type
	if req.MatchType != "contains" && req.MatchType != "regex" && req.MatchType != "exact" {
		return nil, fmt.Errorf("无效的匹配类型: %s", req.MatchType)
	}

	// 编译正则（与 ScanWorkspace 一致）
	var re *regexp.Regexp
	if req.MatchType == "regex" {
		var err error
		re, err = regexp.Compile(req.Pattern)
		if err != nil {
			return nil, fmt.Errorf("无效的正则表达式: %w", err)
		}
	}

	var results []SimulateResult

	// 按文件扫描（模板模式）
	if len(req.Files) > 0 {
		globalLine := 0
		for _, f := range req.Files {
			lines := strings.Split(f.Content, "\n")
			for _, line := range lines {
				globalLine++
				matched := matchLine(line, req.Pattern, req.MatchType, re)
				results = append(results, SimulateResult{
					Line:    globalLine,
					Content: truncateLine(line, 200),
					Matched: matched,
					File:    f.Path,
				})
			}
		}
		return results, nil
	}

	// 纯文本扫描（手动模式）
	lines := strings.Split(req.Content, "\n")
	for i, line := range lines {
		matched := matchLine(line, req.Pattern, req.MatchType, re)
		results = append(results, SimulateResult{
			Line:    i + 1,
			Content: truncateLine(line, 200),
			Matched: matched,
		})
	}
	return results, nil
}

// matchLine 单行匹配 — 与 scanFile 完全一致的逻辑
func matchLine(line, pattern, matchType string, re *regexp.Regexp) bool {
	switch matchType {
	case "contains":
		return strings.Contains(line, pattern)
	case "exact":
		return strings.TrimSpace(line) == pattern
	case "regex":
		if re != nil {
			return re.MatchString(line)
		}
	}
	return false
}

// truncateLine 截断过长行（与 scanFile 一致）
func truncateLine(line string, maxLen int) string {
	if len(line) > maxLen {
		return line[:maxLen] + "..."
	}
	return line
}
