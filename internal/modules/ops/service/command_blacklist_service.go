package service

import (
	"github.com/company/auto-healing/internal/database"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	"gorm.io/gorm"
)

// CommandBlacklistService 高危指令黑名单服务
type CommandBlacklistService struct {
	repo *opsrepo.CommandBlacklistRepository
}

type CommandBlacklistServiceDeps struct {
	Repo *opsrepo.CommandBlacklistRepository
}

// NewCommandBlacklistService 创建服务
func NewCommandBlacklistService() *CommandBlacklistService {
	return NewCommandBlacklistServiceWithDB(database.DB)
}

func NewCommandBlacklistServiceWithDB(db *gorm.DB) *CommandBlacklistService {
	return NewCommandBlacklistServiceWithDeps(CommandBlacklistServiceDeps{
		Repo: opsrepo.NewCommandBlacklistRepositoryWithDB(db),
	})
}

func NewCommandBlacklistServiceWithDeps(deps CommandBlacklistServiceDeps) *CommandBlacklistService {
	return &CommandBlacklistService{
		repo: deps.Repo,
	}
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
