package execution

import (
	"context"
	"fmt"
	"sync"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/engine/provider/ansible"
	"github.com/company/auto-healing/internal/model"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	integrationrepo "github.com/company/auto-healing/internal/modules/integrations/repository"
	"github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/pkg/logger"
	automationrepo "github.com/company/auto-healing/internal/modules/automation/repository"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

const maxExecutionWorkers = 10

// Service 执行任务服务
type Service struct {
	repo             *automationrepo.ExecutionRepository
	gitRepo          *integrationrepo.GitRepositoryRepository
	secretsRepo      *repository.SecretsSourceRepository
	cmdbRepo         *repository.CMDBItemRepository
	healingFlowRepo  *automationrepo.HealingFlowRepository
	workspaceManager *ansible.WorkspaceManager
	localExecutor    *ansible.LocalExecutor
	dockerExecutor   *ansible.DockerExecutor
	notificationSvc  *notification.Service               // 通知服务
	blacklistSvc     *opsservice.CommandBlacklistService // 高危指令扫描
	exemptionSvc     *opsservice.BlacklistExemptionService
	lifecycle        *asyncLifecycle

	// 运行中的执行映射，用于取消操作
	runningExecutions sync.Map // map[uuid.UUID]context.CancelFunc
}

// NewService 创建执行任务服务
func NewService() *Service {
	return &Service{
		repo:             automationrepo.NewExecutionRepository(),
		gitRepo:          integrationrepo.NewGitRepositoryRepository(),
		secretsRepo:      repository.NewSecretsSourceRepository(),
		cmdbRepo:         repository.NewCMDBItemRepository(),
		healingFlowRepo:  automationrepo.NewHealingFlowRepository(),
		workspaceManager: ansible.NewWorkspaceManager(),
		localExecutor:    ansible.NewLocalExecutor(),
		dockerExecutor:   ansible.NewDockerExecutor(),
		notificationSvc:  notification.NewConfiguredService(database.DB),
		blacklistSvc:     opsservice.NewCommandBlacklistService(),
		exemptionSvc:     opsservice.NewBlacklistExemptionService(),
		lifecycle:        newAsyncLifecycle(maxExecutionWorkers),
	}
}

// ==================== 任务模板操作 ====================

// CreateTask 创建任务模板
func (s *Service) CreateTask(ctx context.Context, task *model.ExecutionTask) (*model.ExecutionTask, error) {
	// 通过 PlaybookID 获取 Playbook
	playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
	if err != nil {
		return nil, fmt.Errorf("Playbook 不存在: %w", err)
	}

	// 检查 Playbook 状态
	if playbook.Status != "ready" && playbook.Status != "outdated" {
		return nil, fmt.Errorf("Playbook 状态不可用: %s", playbook.Status)
	}

	if task.ExecutorType == "" {
		task.ExecutorType = "local"
	}

	// 保存 Playbook 当前变量快照
	task.PlaybookVariablesSnapshot = playbook.Variables
	task.NeedsReview = false
	task.ChangedVariables = model.JSONArray{}

	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}

	task.Playbook = playbook
	logger.Exec("TASK").Info("已创建: %s | 名称: %s | Playbook: %s", task.ID, task.Name, playbook.Name)
	return task, nil
}

// GetTask 获取任务模板
func (s *Service) GetTask(ctx context.Context, id uuid.UUID) (*model.ExecutionTask, error) {
	return s.repo.GetTaskByID(ctx, id)
}

// ListTasks 列出任务模板（支持多条件筛选）
func (s *Service) ListTasks(ctx context.Context, opts *automationrepo.TaskListOptions) ([]model.ExecutionTask, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 20
	}
	return s.repo.ListTasks(ctx, opts)
}

// DeleteTask 删除任务模板（保护性删除，执行记录和日志级联删除）
func (s *Service) DeleteTask(ctx context.Context, id uuid.UUID) error {
	_, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return err
	}

	// 检查是否有关联的调度任务
	scheduleCount, err := s.repo.CountSchedulesByTaskID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查关联调度任务失败: %w", err)
	}
	if scheduleCount > 0 {
		return fmt.Errorf("无法删除：该任务模板下有 %d 个调度任务，请先删除关联的调度任务", scheduleCount)
	}

	// 检查是否被自愈流程的 execution 节点引用
	flowCount, err := s.healingFlowRepo.CountFlowsUsingTaskTemplate(ctx, id.String())
	if err != nil {
		return fmt.Errorf("检查关联自愈流程失败: %w", err)
	}
	if flowCount > 0 {
		return fmt.Errorf("无法删除：该任务模板被 %d 个自愈流程使用，请先修改这些流程的执行节点配置", flowCount)
	}

	return s.repo.DeleteTask(ctx, id)
}

// UpdateTask 更新任务模板
func (s *Service) UpdateTask(ctx context.Context, id uuid.UUID, req *model.ExecutionTask) (*model.ExecutionTask, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	logger.Exec("TASK").Info("更新请求: PlaybookID=%s", req.PlaybookID)
	playbookChanged := applyTaskUpdates(task, req)
	if err := s.refreshTaskSnapshotOnPlaybookChange(ctx, task, playbookChanged); err != nil {
		return nil, err
	}

	task.NeedsReview = false
	task.ChangedVariables = model.JSONArray{}
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}

	updatedTask, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	logger.Exec("TASK").Info("已更新: %s | 名称: %s | PlaybookID: %s", updatedTask.ID, updatedTask.Name, updatedTask.PlaybookID)
	return updatedTask, nil
}

func applyTaskUpdates(task, req *model.ExecutionTask) bool {
	playbookChanged := req.PlaybookID != uuid.Nil && req.PlaybookID != task.PlaybookID

	if req.Name != "" {
		task.Name = req.Name
	}
	if req.PlaybookID != uuid.Nil {
		logger.Exec("TASK").Info("更新 PlaybookID: %s -> %s", task.PlaybookID, req.PlaybookID)
		task.PlaybookID = req.PlaybookID
	}
	if req.TargetHosts != "" {
		task.TargetHosts = req.TargetHosts
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.ExecutorType != "" {
		task.ExecutorType = req.ExecutorType
	}
	if req.ExtraVars != nil {
		task.ExtraVars = req.ExtraVars
	}
	if req.NotificationConfig != nil {
		task.NotificationConfig = req.NotificationConfig
	}
	if req.SecretsSourceIDs != nil {
		task.SecretsSourceIDs = req.SecretsSourceIDs
	}

	return playbookChanged
}

func (s *Service) refreshTaskSnapshotOnPlaybookChange(ctx context.Context, task *model.ExecutionTask, playbookChanged bool) error {
	if !playbookChanged {
		return nil
	}

	playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
	if err != nil {
		return fmt.Errorf("Playbook 不存在: %w", err)
	}
	task.PlaybookVariablesSnapshot = playbook.Variables
	return nil
}

// ConfirmReview 确认审核（清除 needs_review 状态）
func (s *Service) ConfirmReview(ctx context.Context, id uuid.UUID) (*model.ExecutionTask, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !task.NeedsReview {
		return task, nil // 不需要审核，直接返回
	}

	// 获取最新的 Playbook 变量并更新快照
	playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
	if err != nil {
		return nil, fmt.Errorf("Playbook 不存在: %w", err)
	}

	task.PlaybookVariablesSnapshot = playbook.Variables
	task.NeedsReview = false
	task.ChangedVariables = model.JSONArray{}

	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}

	logger.Exec("TASK").Info("审核确认: %s | 名称: %s", task.ID, task.Name)
	return s.repo.GetTaskByID(ctx, id)
}
