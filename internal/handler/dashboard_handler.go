package handler

import (
	"context"
	"encoding/json"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DashboardHandler Dashboard 处理器
type DashboardHandler struct {
	repo   *repository.DashboardRepository
	wsRepo *repository.WorkspaceRepository
}

// NewDashboardHandler 创建 Dashboard 处理器
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{
		repo:   repository.NewDashboardRepository(),
		wsRepo: repository.NewWorkspaceRepository(),
	}
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, false
	}
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		return uuid.Nil, false
	}
	return uid, true
}

type dashboardSectionFunc func(context.Context) (interface{}, error)

func dashboardSectionLoader(h *DashboardHandler, section string) dashboardSectionFunc {
	switch section {
	case "incidents":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetIncidentSection(ctx) }
	case "cmdb":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetCMDBSection(ctx) }
	case "healing":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetHealingSection(ctx) }
	case "execution":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetExecutionSection(ctx) }
	case "plugins":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetPluginSection(ctx) }
	case "notifications":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetNotificationSection(ctx) }
	case "git":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetGitSection(ctx) }
	case "playbooks":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetPlaybookSection(ctx) }
	case "secrets":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetSecretsSection(ctx) }
	case "users":
		return func(ctx context.Context) (interface{}, error) { return h.repo.GetUsersSection(ctx) }
	default:
		return nil
	}
}

func parseDashboardBody(c *gin.Context, target interface{}, badRequestMsg string) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		response.BadRequest(c, badRequestMsg+": "+err.Error())
		return false
	}
	return true
}

func toDashboardJSON(body map[string]interface{}) (model.JSON, error) {
	configBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var configJSON model.JSON
	if err := json.Unmarshal(configBytes, &configJSON); err != nil {
		return nil, err
	}
	return configJSON, nil
}
