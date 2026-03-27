package secrets

import "github.com/company/auto-healing/internal/database"

func DefaultModuleDeps() ModuleDeps {
	return DefaultModuleDepsWithDB(database.DB)
}

// New 保留兼容零参构造，生产路径应使用显式 deps。
func New() *Module {
	return NewWithDB(database.DB)
}
