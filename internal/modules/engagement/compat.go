package engagement

import "github.com/company/auto-healing/internal/database"

// New 保留给兼容调用方；生产主路径应使用 NewWithDB。
func New() *Module {
	return NewWithDB(database.DB)
}
