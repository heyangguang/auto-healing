package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/company/auto-healing/internal/modules/ops/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	"github.com/google/uuid"
)

// Create 创建规则
func (s *CommandBlacklistService) Create(ctx context.Context, rule *model.CommandBlacklist) error {
	rule.ID = uuid.New()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.IsSystem = false
	if err := opsrepo.FillTenantID(ctx, &rule.TenantID); err != nil {
		return err
	}
	if err := validateCommandBlacklistRule(rule.MatchType, rule.Pattern, rule.Severity); err != nil {
		return err
	}
	if rule.MatchType == "" {
		rule.MatchType = "contains"
	}
	if rule.Severity == "" {
		rule.Severity = "critical"
	}
	return s.repo.Create(ctx, rule)
}

func validateCommandBlacklistRule(matchType, pattern, severity string) error {
	if matchType == "" {
		matchType = "contains"
	}
	if matchType != "contains" && matchType != "regex" && matchType != "exact" {
		return fmt.Errorf("无效的匹配类型: %s, 支持 contains/regex/exact", matchType)
	}
	if matchType == "regex" {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("无效的正则表达式: %w", err)
		}
	}
	if severity == "" {
		severity = "critical"
	}
	if severity != "critical" && severity != "high" && severity != "medium" {
		return fmt.Errorf("无效的严重级别: %s, 支持 critical/high/medium", severity)
	}
	return nil
}

// GetByID 获取规则
func (s *CommandBlacklistService) GetByID(ctx context.Context, id uuid.UUID) (*model.CommandBlacklist, error) {
	return s.repo.GetByID(ctx, id)
}

// List 列表查询
func (s *CommandBlacklistService) List(ctx context.Context, opts *opsrepo.CommandBlacklistListOptions) ([]model.CommandBlacklist, int64, error) {
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
	applyCommandBlacklistUpdate(rule, input)
	if err := validateCommandBlacklistRule(rule.MatchType, rule.Pattern, rule.Severity); err != nil {
		return nil, err
	}
	rule.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func applyCommandBlacklistUpdate(rule, input *model.CommandBlacklist) {
	if input.Name != "" {
		rule.Name = input.Name
	}
	if input.Pattern != "" {
		rule.Pattern = input.Pattern
	}
	if input.MatchType != "" {
		rule.MatchType = input.MatchType
	}
	if input.Severity != "" {
		rule.Severity = input.Severity
	}
	if input.Category != "" {
		rule.Category = input.Category
	}
	if input.Description != "" {
		rule.Description = input.Description
	}
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

// ToggleActive 启用/禁用规则
func (s *CommandBlacklistService) ToggleActive(ctx context.Context, id uuid.UUID) (*model.CommandBlacklist, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rule.IsSystem || rule.TenantID == nil {
		newActive := !rule.IsActive
		if err := s.repo.ToggleSystemRule(ctx, id, newActive); err != nil {
			return nil, err
		}
		rule.IsActive = newActive
		return rule, nil
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

// GetActiveRules 获取所有启用的规则（用于外部调用）
func (s *CommandBlacklistService) GetActiveRules(ctx context.Context) ([]model.CommandBlacklist, error) {
	return s.repo.GetActiveRules(ctx)
}
