package access

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
)

// New 保留兼容入口，主实现统一走显式 WithDB/WithDeps。
func New(cfg *config.Config) *Module {
	return NewWithDB(cfg, database.DB)
}
