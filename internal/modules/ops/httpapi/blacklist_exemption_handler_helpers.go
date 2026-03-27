package httpapi

import (
	"errors"
	"strings"

	"github.com/company/auto-healing/internal/middleware"
	automationmodel "github.com/company/auto-healing/internal/modules/automation/model"
	opsmodel "github.com/company/auto-healing/internal/modules/ops/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type blacklistExemptionCreateInput struct {
	TaskID       string `json:"task_id" binding:"required"`
	TaskName     string `json:"task_name"`
	RuleID       string `json:"rule_id" binding:"required"`
	RuleName     string `json:"rule_name"`
	RuleSeverity string `json:"rule_severity"`
	RulePattern  string `json:"rule_pattern"`
	Reason       string `json:"reason" binding:"required"`
	ValidityDays int    `json:"validity_days"`
}

func parseBlacklistExemptionCreateInput(c *gin.Context) (*blacklistExemptionCreateInput, bool) {
	var input blacklistExemptionCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return nil, false
	}
	if input.ValidityDays <= 0 {
		input.ValidityDays = 30
	}
	return &input, true
}

func parseBlacklistExemptionIDs(c *gin.Context, taskIDValue, ruleIDValue string) (uuid.UUID, uuid.UUID, bool) {
	taskID, err := uuid.Parse(taskIDValue)
	if err != nil {
		response.BadRequest(c, "无效的任务模板 ID")
		return uuid.Nil, uuid.Nil, false
	}
	ruleID, err := uuid.Parse(ruleIDValue)
	if err != nil {
		response.BadRequest(c, "无效的规则 ID")
		return uuid.Nil, uuid.Nil, false
	}
	return taskID, ruleID, true
}

func (h *BlacklistExemptionHandler) loadBlacklistExemptionDependencies(c *gin.Context, taskID, ruleID uuid.UUID) (*automationmodel.ExecutionTask, *opsmodel.CommandBlacklist, bool) {
	task, err := h.taskRepo.GetTaskByID(c.Request.Context(), taskID)
	if err != nil {
		respondBlacklistExemptionDependencyError(c, err, "任务模板不存在或不属于当前租户", "查询任务模板失败")
		return nil, nil, false
	}
	rule, err := h.blacklistRepo.GetByID(c.Request.Context(), ruleID)
	if err != nil {
		respondBlacklistExemptionDependencyError(c, err, "黑名单规则不存在或不属于当前租户", "查询黑名单规则失败")
		return nil, nil, false
	}
	return task, rule, true
}

func buildBlacklistExemptionModel(c *gin.Context, userID uuid.UUID, input *blacklistExemptionCreateInput, taskID, ruleID uuid.UUID, taskName, ruleName, ruleSeverity, rulePattern string) *opsmodel.BlacklistExemption {
	return &opsmodel.BlacklistExemption{
		TaskID:        taskID,
		TaskName:      taskName,
		RuleID:        ruleID,
		RuleName:      ruleName,
		RuleSeverity:  ruleSeverity,
		RulePattern:   rulePattern,
		Reason:        input.Reason,
		RequestedBy:   userID,
		RequesterName: middleware.GetUsername(c),
		ValidityDays:  input.ValidityDays,
	}
}

func requireBlacklistExemptionUserID(c *gin.Context) (uuid.UUID, bool) {
	userID := parseBlacklistExemptionUserID(middleware.GetUserID(c))
	if userID == uuid.Nil {
		respondInternalError(c, "BLACKLIST", "用户上下文缺失", errors.New("invalid user id in context"))
		return uuid.Nil, false
	}
	return userID, true
}

func parseBlacklistExemptionUserID(raw string) uuid.UUID {
	userID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}
	return userID
}

func respondBlacklistExemptionLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "豁免申请不存在")
		return
	}
	respondInternalError(c, "BLACKLIST", "查询豁免申请失败", err)
}

func respondBlacklistExemptionDependencyError(c *gin.Context, err error, notFoundMsg, internalMsg string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.BadRequest(c, notFoundMsg)
		return
	}
	respondInternalError(c, "BLACKLIST", internalMsg, err)
}

func respondBlacklistExemptionMutationError(c *gin.Context, err error, internalMsg string) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.NotFound(c, "豁免申请不存在")
	case errors.Is(err, opsrepo.ErrBlacklistExemptionNotPending):
		response.Conflict(c, "该豁免申请已被其他审批人处理")
	case errors.Is(err, platformrepo.ErrTenantContextRequired):
		respondInternalError(c, "BLACKLIST", internalMsg, err)
	default:
		if isBlacklistExemptionBadRequest(err) {
			response.BadRequest(c, err.Error())
			return
		}
		respondInternalError(c, "BLACKLIST", internalMsg, err)
	}
}

func isBlacklistExemptionBadRequest(err error) bool {
	if err == nil {
		return false
	}
	for _, prefix := range []string{
		"该任务模板已有相同规则的待审批豁免申请",
		"只有待审批的申请才能审批",
		"申请人不能审批自己的豁免申请",
	} {
		if strings.HasPrefix(err.Error(), prefix) {
			return true
		}
	}
	return false
}
