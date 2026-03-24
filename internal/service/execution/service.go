package execution

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/engine/provider/ansible"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/secrets"
	parentService "github.com/company/auto-healing/internal/service"
	"github.com/google/uuid"
)

// Service 执行任务服务
type Service struct {
	repo             *repository.ExecutionRepository
	gitRepo          *repository.GitRepositoryRepository
	secretsRepo      *repository.SecretsSourceRepository
	cmdbRepo         *repository.CMDBItemRepository
	healingFlowRepo  *repository.HealingFlowRepository
	workspaceManager *ansible.WorkspaceManager
	localExecutor    *ansible.LocalExecutor
	dockerExecutor   *ansible.DockerExecutor
	notificationSvc  *notification.Service                  // 通知服务
	blacklistSvc     *parentService.CommandBlacklistService // 高危指令扫描
	exemptionSvc     *parentService.BlacklistExemptionService

	// 运行中的执行映射，用于取消操作
	runningExecutions sync.Map // map[uuid.UUID]context.CancelFunc
}

// NewService 创建执行任务服务
func NewService() *Service {
	return &Service{
		repo:             repository.NewExecutionRepository(),
		gitRepo:          repository.NewGitRepositoryRepository(),
		secretsRepo:      repository.NewSecretsSourceRepository(),
		cmdbRepo:         repository.NewCMDBItemRepository(),
		healingFlowRepo:  repository.NewHealingFlowRepository(),
		workspaceManager: ansible.NewWorkspaceManager(),
		localExecutor:    ansible.NewLocalExecutor(),
		dockerExecutor:   ansible.NewDockerExecutor(),
		notificationSvc:  notification.NewService(database.DB, "Auto-Healing", "", "1.0.0"),
		blacklistSvc:     parentService.NewCommandBlacklistService(),
		exemptionSvc:     parentService.NewBlacklistExemptionService(),
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
func (s *Service) ListTasks(ctx context.Context, opts *repository.TaskListOptions) ([]model.ExecutionTask, int64, error) {
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
	// 1. 获取现有任务
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	logger.Exec("TASK").Info("更新请求: PlaybookID=%s", req.PlaybookID)

	// 检查 Playbook ID 是否变更
	playbookChanged := req.PlaybookID != uuid.Nil && req.PlaybookID != task.PlaybookID

	// 2. 更新字段 (仅更新非零值/非空值)
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

	// 3. 如果 Playbook ID 变更，更新变量快照
	if playbookChanged {
		playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
		if err != nil {
			return nil, fmt.Errorf("Playbook 不存在: %w", err)
		}
		task.PlaybookVariablesSnapshot = playbook.Variables
	}

	// 4. 用户保存即表示审核完成，清除 review 状态
	task.NeedsReview = false
	task.ChangedVariables = model.JSONArray{}

	// 5. 保存更新
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}

	// 6. 重新加载任务以获取更新后的 Playbook 关联
	updatedTask, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	logger.Exec("TASK").Info("已更新: %s | 名称: %s | PlaybookID: %s", updatedTask.ID, updatedTask.Name, updatedTask.PlaybookID)
	return updatedTask, nil
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

// ==================== 执行操作 ====================

// ExecuteOptions 执行选项
type ExecuteOptions struct {
	TriggeredBy      string
	SecretsSourceIDs []uuid.UUID
	ExtraVars        map[string]any
	TargetHosts      string // 覆盖目标主机
	SkipNotification bool   // 跳过本次通知（全局）
}

// ExecuteTask 异步执行任务（立即返回 RunID，后台执行）
func (s *Service) ExecuteTask(ctx context.Context, taskID uuid.UUID, opts *ExecuteOptions) (*model.ExecutionRun, error) {
	// 获取任务模板
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("任务不存在: %w", err)
	}

	// 检查是否需要审核变量变更
	if task.NeedsReview {
		return nil, fmt.Errorf("任务模板需要审核变量变更后才能执行，变更字段: %v", task.ChangedVariables)
	}

	// 处理 target_hosts 覆盖
	targetHosts := task.TargetHosts
	if opts.TargetHosts != "" {
		targetHosts = opts.TargetHosts
		logger.Exec("TASK").Info("使用运行时目标主机: %s", targetHosts)
	}

	// 通过 PlaybookID 获取 Playbook
	playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
	if err != nil {
		return nil, fmt.Errorf("Playbook 不存在: %w", err)
	}

	// 检查 Playbook 状态：只有 ready 状态才能执行
	if playbook.Status != "ready" {
		return nil, fmt.Errorf("Playbook 未上线，无法执行 (当前状态: %s)", playbook.Status)
	}

	// 通过 Playbook 获取仓库
	gitRepo, err := s.gitRepo.GetByID(ctx, playbook.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("仓库不存在: %w", err)
	}

	// 默认触发者
	triggeredBy := opts.TriggeredBy
	if triggeredBy == "" {
		triggeredBy = "manual"
	}

	// 密钥源优先级：运行时覆盖 > 任务模板默认配置
	secretsSourceIDs := opts.SecretsSourceIDs
	if len(secretsSourceIDs) == 0 && len(task.SecretsSourceIDs) > 0 {
		for _, idStr := range task.SecretsSourceIDs {
			if id, err := uuid.Parse(idStr); err == nil {
				secretsSourceIDs = append(secretsSourceIDs, id)
			}
		}
	}

	run := &model.ExecutionRun{
		TaskID:      taskID,
		Status:      "pending",
		TriggeredBy: triggeredBy,
		// 运行时参数快照
		RuntimeTargetHosts:      targetHosts,
		RuntimeSecretsSourceIDs: uuidsToStrings(secretsSourceIDs),
		RuntimeExtraVars:        toJSON(opts.ExtraVars),
		RuntimeSkipNotification: opts.SkipNotification,
	}

	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("创建执行记录失败: %w", err)
	}

	// 构建执行参数（使用覆盖后的 targetHosts）
	execOpts := &executeParams{
		targetHosts:      targetHosts,
		extraVars:        opts.ExtraVars,
		secretsSourceIDs: secretsSourceIDs,
		skipNotification: opts.SkipNotification,
	}

	// 后台异步执行（传入任务的租户ID，确保 secrets/CMDB 等在正确租户下查询）
	go s.executeInBackground(run.ID, task, playbook, gitRepo, execOpts, task.TenantID)

	// 立即返回执行记录
	return run, nil
}

// executeParams 执行参数
type executeParams struct {
	targetHosts      string
	extraVars        map[string]any
	secretsSourceIDs []uuid.UUID
	skipNotification bool
}

// executeInBackground 后台执行任务
func (s *Service) executeInBackground(runID uuid.UUID, task *model.ExecutionTask, playbook *model.Playbook, gitRepo *model.GitRepository, params *executeParams, taskTenantID *uuid.UUID) {
	// 创建带租户上下文的可取消 context
	// 注入任务的 TenantID，确保 secrets/CMDB/日志写入均在正确租户范围内操作
	baseCtx := context.Background()
	if taskTenantID != nil {
		baseCtx = repository.WithTenantID(baseCtx, *taskTenantID)
	}
	ctx, cancel := context.WithCancel(baseCtx)

	// 注册取消函数，用于取消操作
	s.runningExecutions.Store(runID, cancel)
	defer func() {
		s.runningExecutions.Delete(runID)
		cancel() // 确保资源释放
	}()

	// panic 保护：防止 panic 导致执行记录永远停留在 running 状态
	defer func() {
		if rec := recover(); rec != nil {
			logger.Exec("RUN").Error("[%s] executeInBackground panic: %v", runID.String()[:8], rec)
			s.repo.UpdateRunResult(ctx, runID, -1, "", fmt.Sprintf("内部错误: %v", rec), nil)
		}
	}()

	// 更新状态为 running
	started, err := s.repo.UpdateRunStarted(ctx, runID)
	if err != nil {
		logger.Exec("RUN").Error("[%s] 更新执行开始状态失败: %v", runID.String()[:8], err)
		return
	}
	if !started {
		logger.Exec("RUN").Warn("[%s] 执行在启动前已取消，跳过后台执行", runID.String()[:8])
		return
	}

	// 发送开始通知（如果配置了且未跳过）
	if !params.skipNotification && task.NotificationConfig != nil && task.NotificationConfig.Enabled {
		run, err := s.repo.GetRunByID(ctx, runID)
		if err == nil {
			task.Playbook = playbook
			if logs, err := s.notificationSvc.SendOnStart(ctx, run, task); err != nil {
				s.appendLog(ctx, runID, "warn", "notification", fmt.Sprintf("发送开始通知失败: %v", err), nil)
			} else if len(logs) > 0 {
				s.appendLog(ctx, runID, "info", "notification", fmt.Sprintf("已发送开始通知: %d 条", len(logs)), nil)
			}
		}
	}

	// 记录开始日志
	s.appendLog(ctx, runID, "info", "prepare", "开始准备执行环境", nil)

	// 准备工作空间
	workDir, cleanup, err := s.workspaceManager.PrepareWorkspace(runID, gitRepo.LocalPath)
	if err != nil {
		s.repo.UpdateRunResult(ctx, runID, -1, "", err.Error(), nil)
		s.appendLog(ctx, runID, "error", "prepare", fmt.Sprintf("准备工作空间失败: %v", err), nil)
		return
	}
	defer cleanup()

	s.appendLog(ctx, runID, "info", "prepare", fmt.Sprintf("工作空间已准备: %s", workDir), nil)

	// ======= 高危指令扫描 =======
	s.appendLog(ctx, runID, "info", "security", "开始安全扫描...", nil)
	violations, scanErr := s.blacklistSvc.ScanWorkspace(ctx, workDir)
	if scanErr != nil {
		s.appendLog(ctx, runID, "warn", "security", fmt.Sprintf("安全扫描异常: %v", scanErr), nil)
	}
	if len(violations) > 0 {
		approvedExemptions, err := s.exemptionSvc.GetApprovedByTaskID(ctx, task.ID)
		if err != nil {
			s.appendLog(ctx, runID, "warn", "security", fmt.Sprintf("加载豁免规则失败: %v", err), nil)
		} else if len(approvedExemptions) > 0 {
			logger.Exec("SECURITY").Info("[%s] 发现 %d 条已批准豁免规则", runID.String()[:8], len(approvedExemptions))
			exemptedRuleIDs := make(map[uuid.UUID]bool, len(approvedExemptions))
			for _, item := range approvedExemptions {
				exemptedRuleIDs[item.RuleID] = true
				logger.Exec("SECURITY").Info("[%s] 豁免规则: id=%s name=%s pattern=%s", runID.String()[:8], item.RuleID, item.RuleName, item.RulePattern)
			}

			filtered := violations[:0]
			exemptedCount := 0
			for _, violation := range violations {
				logger.Exec("SECURITY").Info("[%s] 违规命中: id=%s name=%s pattern=%s", runID.String()[:8], violation.RuleID, violation.RuleName, violation.Pattern)
				if exemptedRuleIDs[violation.RuleID] {
					exemptedCount++
					continue
				}
				filtered = append(filtered, violation)
			}
			violations = filtered
			if exemptedCount > 0 {
				logger.Exec("SECURITY").Info("[%s] 已应用 %d 条豁免规则", runID.String()[:8], exemptedCount)
				s.appendLog(ctx, runID, "info", "security", fmt.Sprintf("已应用 %d 条豁免规则，跳过对应安全拦截", exemptedCount), nil)
			}
		}
	}
	if len(violations) > 0 {
		// 构造违规详情
		var violationList []map[string]interface{}
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("检测到 %d 个高危指令，执行已拦截:\n", len(violations)))
		for i, v := range violations {
			violationList = append(violationList, map[string]interface{}{
				"rule_id":   v.RuleID,
				"file":      v.File,
				"line":      v.Line,
				"content":   v.Content,
				"rule_name": v.RuleName,
				"pattern":   v.Pattern,
				"severity":  v.Severity,
			})
			msg.WriteString(fmt.Sprintf("  %d. [%s] %s (文件: %s, 行: %d)\n", i+1, v.Severity, v.RuleName, v.File, v.Line))
		}
		// 记录详细日志
		s.appendLog(ctx, runID, "error", "security", msg.String(), model.JSON{
			"violations": violationList,
			"count":      len(violations),
		})
		// 拒绝执行，退出码 -2 标识安全拦截
		s.repo.UpdateRunResult(ctx, runID, -2, "", fmt.Sprintf("安全拦截：检测到 %d 个高危指令", len(violations)), nil)
		logger.Exec("SECURITY").Warn("[%s] 检测到 %d 个高危指令，执行已拦截", runID.String()[:8], len(violations))
		return
	}
	s.appendLog(ctx, runID, "info", "security", "安全扫描通过", nil)

	// 生成 inventory（带或不带认证）
	var inventoryPath string
	if len(params.secretsSourceIDs) > 0 {
		// 加载所有密钥源和提供者
		type sourceProvider struct {
			source   *model.SecretsSource
			provider secrets.Provider
		}
		var providers []sourceProvider

		for _, sourceID := range params.secretsSourceIDs {
			source, err := s.secretsRepo.GetByID(ctx, sourceID)
			if err != nil {
				s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("获取密钥源 %s 失败: %v", sourceID, err), nil)
				continue
			}
			if source.Status != "active" {
				s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("密钥源 %s 未启用，已跳过", source.Name), nil)
				continue
			}
			provider, err := secrets.NewProvider(source)
			if err != nil {
				s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("创建密钥提供者失败: %v", err), nil)
				continue
			}
			providers = append(providers, sourceProvider{source: source, provider: provider})
			s.appendLog(ctx, runID, "info", "prepare", fmt.Sprintf("使用密钥源: %s (类型: %s, 认证: %s)", source.Name, source.Type, source.AuthType), nil)
		}

		if len(providers) == 0 {
			s.repo.UpdateRunResult(ctx, runID, -1, "", "没有可用的密钥源", nil)
			s.appendLog(ctx, runID, "error", "prepare", "没有可用的密钥源", nil)
			return
		}

		// 为每台主机获取凭据（依次尝试每个密钥源）
		hostList := strings.Split(params.targetHosts, ",")
		var credentials []ansible.HostCredential

		for _, host := range hostList {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}

			var cred *ansible.HostCredential

			// 依次尝试每个密钥源
			for _, sp := range providers {
				// 从 CMDB 获取真实的 IP 和 hostname
				var ipAddress, hostname string
				cmdbItem, cmdbErr := s.cmdbRepo.FindByNameOrIP(ctx, host)
				if cmdbErr == nil {
					ipAddress = cmdbItem.IPAddress
					hostname = cmdbItem.Hostname
					if hostname == "" {
						hostname = cmdbItem.Name
					}
				} else {
					// CMDB 中找不到，使用原始 host（兼容处理）
					ipAddress = host
					hostname = host
				}

				query := model.SecretQuery{
					Hostname:  hostname,
					IPAddress: ipAddress,
					AuthType:  sp.source.AuthType,
				}

				secret, err := sp.provider.GetSecret(ctx, query)
				if err != nil {
					// 这个密钥源没有该主机的凭据，尝试下一个
					continue
				}

				// 找到凭据
				cred = &ansible.HostCredential{
					Host:     host,
					AuthType: secret.AuthType,
					Username: secret.Username,
				}

				// 如果是 ssh_key，写入临时文件
				if secret.AuthType == "ssh_key" && secret.PrivateKey != "" {
					keyFileName := fmt.Sprintf("key_%s", strings.ReplaceAll(host, ".", "_"))
					keyPath, err := ansible.WriteKeyFile(workDir, keyFileName, secret.PrivateKey)
					if err != nil {
						s.appendLog(ctx, runID, "error", "prepare", fmt.Sprintf("写入密钥文件失败: %v", err), nil)
						return
					}
					// Docker 执行器使用容器内路径
					if task.ExecutorType == "docker" {
						cred.KeyFile = "/workspace/" + keyFileName
					} else {
						cred.KeyFile = keyPath
					}
				} else if secret.AuthType == "password" {
					cred.Password = secret.Password
				}

				s.appendLog(ctx, runID, "debug", "prepare", fmt.Sprintf("主机 %s 使用密钥源 %s (%s)", host, sp.source.Name, sp.source.AuthType), nil)
				break // 找到凭据，跳出密钥源循环
			}

			if cred != nil {
				credentials = append(credentials, *cred)
			} else {
				// 所有密钥源都没有该主机的凭据
				s.appendLog(ctx, runID, "warn", "prepare", fmt.Sprintf("主机 %s 在所有密钥源中都未找到凭据，将使用默认认证", host), nil)
				credentials = append(credentials, ansible.HostCredential{Host: host})
			}
		}

		// 生成带认证的 inventory
		inventoryContent := ansible.GenerateInventoryWithAuth(credentials, "targets")
		inventoryPath, err = ansible.WriteInventoryFile(workDir, inventoryContent)
		if err != nil {
			s.repo.UpdateRunResult(ctx, runID, -1, "", err.Error(), nil)
			s.appendLog(ctx, runID, "error", "prepare", fmt.Sprintf("生成 inventory 失败: %v", err), nil)
			return
		}

		s.appendLog(ctx, runID, "info", "prepare", fmt.Sprintf("Inventory 已生成（含 %d 台主机认证信息）", len(credentials)), nil)
	} else {
		// 不使用密钥源，生成简单 inventory
		inventoryContent := ansible.GenerateInventory(params.targetHosts, "targets", nil)
		inventoryPath, err = ansible.WriteInventoryFile(workDir, inventoryContent)
		if err != nil {
			s.repo.UpdateRunResult(ctx, runID, -1, "", err.Error(), nil)
			s.appendLog(ctx, runID, "error", "prepare", fmt.Sprintf("生成 inventory 失败: %v", err), nil)
			return
		}

		s.appendLog(ctx, runID, "info", "prepare", "Inventory 已生成", nil)
	}

	// 合并变量
	mergedVars := map[string]any(task.ExtraVars)
	for k, v := range params.extraVars {
		mergedVars[k] = v
	}

	// 构建执行请求
	execReq := &ansible.ExecuteRequest{
		PlaybookPath: playbook.FilePath,
		WorkDir:      workDir,
		Inventory:    inventoryPath,
		ExtraVars:    mergedVars,
		Timeout:      30 * time.Minute,
		// 实时日志回调
		LogCallback: func(level, stage, message string) {
			s.appendLog(ctx, runID, level, stage, message, nil)
		},
	}

	// 选择执行器
	var executor ansible.Executor
	if task.ExecutorType == "docker" {
		executor = s.dockerExecutor
	} else {
		executor = s.localExecutor
	}

	s.appendLog(ctx, runID, "info", "execute", fmt.Sprintf("开始执行 Playbook (执行器: %s)", executor.Name()), nil)

	// 执行
	result, execErr := executor.Execute(ctx, execReq)

	// 记录执行结果
	var stats model.JSON
	if result != nil && result.Stats != nil {
		stats = model.JSON{
			"ok":          result.Stats.Ok,
			"changed":     result.Stats.Changed,
			"unreachable": result.Stats.Unreachable,
			"failed":      result.Stats.Failed,
			"skipped":     result.Stats.Skipped,
			"rescued":     result.Stats.Rescued,
			"ignored":     result.Stats.Ignored,
		}
	}

	exitCode := -1
	stdout := ""
	stderr := ""
	if result != nil {
		exitCode = result.ExitCode
		stdout = result.Stdout
		stderr = result.Stderr
	}

	// 检查是否被取消 - 如果 context 已取消，保留 cancelled 状态
	if ctx.Err() != nil {
		logger.Exec("RUN").Warn("执行被取消，跳过结果更新: %s", runID)
		return
	}

	s.repo.UpdateRunResult(ctx, runID, exitCode, stdout, stderr, stats)

	// runID 短码用于区分不同执行
	shortID := runID.String()[:8]

	// 将 Ansible 输出打印到终端（仅用于调试，日志已通过 LogCallback 实时写入数据库）
	if stdout != "" {
		for _, line := range strings.Split(stdout, "\n") {
			if line != "" {
				logger.Exec("ANSIBLE").Info("[%s] %s", shortID, line)
			}
		}
		// 注意：不再重复写入数据库，因为 LogCallback 已经实时写入了每一行
	}
	if stderr != "" {
		for _, line := range strings.Split(stderr, "\n") {
			if line != "" {
				logger.Exec("ANSIBLE").Warn("[%s] %s", shortID, line)
			}
		}
		// 注意：不再重复写入数据库
	}

	// 记录完成日志
	if execErr != nil {
		s.appendLog(ctx, runID, "error", "execute", fmt.Sprintf("执行失败: %v", execErr), model.JSON{
			"exit_code": exitCode,
			"stats":     stats,
		})
	} else if exitCode == 0 {
		s.appendLog(ctx, runID, "info", "execute", fmt.Sprintf("执行成功 (耗时: %v)", result.Duration), model.JSON{
			"exit_code": exitCode,
			"stats":     stats,
		})
	} else {
		s.appendLog(ctx, runID, "warn", "execute", fmt.Sprintf("执行完成但有错误 (退出码: %d)", exitCode), model.JSON{
			"exit_code": exitCode,
			"stats":     stats,
		})
	}

	// 定时任务的下次执行时间更新已移至 ExecutionSchedule 模块处理

	logger.Exec("RUN").Info("完成: %s | 状态: %s | 退出码: %d", runID, getStatusFromExitCode(exitCode), exitCode)

	// 发送通知（如果任务配置了通知且未跳过）
	if params.skipNotification {
		s.appendLog(ctx, runID, "info", "notification", "本次执行跳过通知", nil)
	} else if task.NotificationConfig != nil && task.NotificationConfig.Enabled {
		run, err := s.repo.GetRunByID(ctx, runID)
		if err == nil {
			// 加载 Playbook 信息用于通知变量
			task.Playbook = playbook
			logs, err := s.notificationSvc.SendFromExecution(ctx, run, task)
			if err != nil {
				s.appendLog(ctx, runID, "warn", "notification", fmt.Sprintf("发送通知失败: %v", err), nil)
			} else if len(logs) > 0 {
				s.appendLog(ctx, runID, "info", "notification", fmt.Sprintf("已发送 %d 条通知", len(logs)), nil)
			}
		}
	}
}

// GetRun 获取执行记录
func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (*model.ExecutionRun, error) {
	return s.repo.GetRunByID(ctx, id)
}

// GetRunsByTaskID 获取任务的执行历史
func (s *Service) GetRunsByTaskID(ctx context.Context, taskID uuid.UUID, page, pageSize int) ([]model.ExecutionRun, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListRunsByTaskID(ctx, taskID, page, pageSize)
}

// ListAllRuns 获取所有执行记录列表（跨任务模板）
func (s *Service) ListAllRuns(ctx context.Context, opts *repository.RunListOptions) ([]model.ExecutionRun, int64, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 20
	}
	return s.repo.ListAllRuns(ctx, opts)
}

// GetRunLogs 获取执行日志
func (s *Service) GetRunLogs(ctx context.Context, runID uuid.UUID) ([]model.ExecutionLog, error) {
	return s.repo.GetLogsByRunID(ctx, runID)
}

// CancelRun 取消执行
func (s *Service) CancelRun(ctx context.Context, id uuid.UUID) error {
	run, err := s.repo.GetRunByID(ctx, id)
	if err != nil {
		return err
	}

	if run.Status != "pending" && run.Status != "running" {
		return fmt.Errorf("执行状态不允许取消: %s", run.Status)
	}

	// 调用取消函数终止进程
	if cancelFunc, ok := s.runningExecutions.Load(id); ok {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			cancel()
			logger.Exec("RUN").Warn("已发送取消信号: %s", id)
		}
		s.runningExecutions.Delete(id)
	}

	if err := s.repo.UpdateRunStatus(ctx, id, "cancelled"); err != nil {
		return err
	}
	s.appendLog(ctx, id, "warn", "control", "执行已被取消", nil)

	logger.Exec("RUN").Warn("已取消: %s", id)
	return nil
}

// ==================== 统计方法 ====================

// GetRunStats 获取执行记录统计概览
func (s *Service) GetRunStats(ctx context.Context) (*repository.RunStats, error) {
	return s.repo.GetRunStats(ctx)
}

// GetRunTrend 获取执行趋势
func (s *Service) GetRunTrend(ctx context.Context, days int) ([]repository.RunTrendItem, error) {
	return s.repo.GetRunTrend(ctx, days)
}

// GetTriggerDistribution 获取触发方式分布
func (s *Service) GetTriggerDistribution(ctx context.Context) ([]repository.TriggerDistItem, error) {
	return s.repo.GetTriggerDistribution(ctx)
}

// GetTopFailedTasks 获取失败率最高的 Top N 任务
func (s *Service) GetTopFailedTasks(ctx context.Context, limit int) ([]repository.TaskFailRate, error) {
	return s.repo.GetTopFailedTasks(ctx, limit)
}

// GetTopActiveTasks 获取最活跃的 Top N 任务
func (s *Service) GetTopActiveTasks(ctx context.Context, limit int) ([]repository.TaskActivity, error) {
	return s.repo.GetTopActiveTasks(ctx, limit)
}

// GetTaskStats 获取任务模板统计概览
func (s *Service) GetTaskStats(ctx context.Context) (*repository.TaskStats, error) {
	return s.repo.GetTaskStats(ctx)
}

// BatchConfirmReviewRequest 批量审核请求
type BatchConfirmReviewRequest struct {
	TaskIDs    []uuid.UUID `json:"task_ids"`
	PlaybookID *uuid.UUID  `json:"playbook_id"`
}

// BatchConfirmReviewResponse 批量审核响应
type BatchConfirmReviewResponse struct {
	ConfirmedCount int64  `json:"confirmed_count"`
	Message        string `json:"message"`
}

// BatchConfirmReview 批量确认审核（同时更新快照）
func (s *Service) BatchConfirmReview(ctx context.Context, req *BatchConfirmReviewRequest) (*BatchConfirmReviewResponse, error) {
	// 1. 查出需要审核的任务
	var tasks []model.ExecutionTask
	var err error

	if req.PlaybookID != nil {
		tasks, err = s.repo.ListTasksByPlaybookIDAndReview(ctx, *req.PlaybookID)
	} else if len(req.TaskIDs) > 0 {
		tasks, err = s.repo.ListTasksByIDsAndReview(ctx, req.TaskIDs)
	} else {
		return nil, fmt.Errorf("必须提供 task_ids 或 playbook_id")
	}

	if err != nil {
		return nil, err
	}

	if len(tasks) == 0 {
		return &BatchConfirmReviewResponse{
			ConfirmedCount: 0,
			Message:        "没有需要确认的任务",
		}, nil
	}

	// 2. 按 PlaybookID 分组，获取最新的 Playbook 变量
	playbookVarsCache := make(map[uuid.UUID]model.JSONArray)
	for _, task := range tasks {
		if _, cached := playbookVarsCache[task.PlaybookID]; !cached {
			playbook, err := s.repo.GetPlaybookByID(ctx, task.PlaybookID)
			if err != nil {
				logger.Exec("TASK").Warn("获取 Playbook %s 失败: %v", task.PlaybookID, err)
				playbookVarsCache[task.PlaybookID] = nil // 标记为获取失败
				continue
			}
			playbookVarsCache[task.PlaybookID] = playbook.Variables
		}
	}

	// 3. 逐个更新任务（清除审核状态 + 同步快照）
	var count int64
	for _, task := range tasks {
		vars := playbookVarsCache[task.PlaybookID]
		if vars != nil {
			task.PlaybookVariablesSnapshot = vars
		}
		task.NeedsReview = false
		task.ChangedVariables = model.JSONArray{}

		if err := s.repo.UpdateTask(ctx, &task); err != nil {
			logger.Exec("TASK").Warn("批量审核更新任务 %s 失败: %v", task.ID, err)
			continue
		}
		count++
	}

	logger.Exec("TASK").Info("批量审核确认: %d 个任务模板（快照已同步）", count)
	return &BatchConfirmReviewResponse{
		ConfirmedCount: count,
		Message:        fmt.Sprintf("已确认 %d 个任务模板", count),
	}, nil
}

// ==================== 内部方法 ====================

// appendLog 追加日志
func (s *Service) appendLog(ctx context.Context, runID uuid.UUID, level, stage, message string, details map[string]any) {
	seq, _ := s.repo.GetNextLogSequence(ctx, runID)

	var detailsJSON model.JSON
	if details != nil {
		detailsJSON = model.JSON(details)
	}

	log := &model.ExecutionLog{
		RunID:    runID,
		LogLevel: level,
		Stage:    stage,
		Message:  message,
		Details:  detailsJSON,
		Sequence: seq,
	}

	s.repo.AppendLog(ctx, log)
}

// getStatusFromExitCode 根据退出码获取状态
func getStatusFromExitCode(exitCode int) string {
	if exitCode == 0 {
		return "success"
	}
	return "failed"
}

// uuidsToStrings 将 UUID 列表转换为字符串列表
func uuidsToStrings(uuids []uuid.UUID) model.StringArray {
	result := make(model.StringArray, len(uuids))
	for i, u := range uuids {
		result[i] = u.String()
	}
	return result
}

// toJSON 将 map 转换为 model.JSON
func toJSON(m map[string]any) model.JSON {
	if m == nil {
		return nil
	}
	result := make(model.JSON)
	for k, v := range m {
		result[k] = v
	}
	return result
}
