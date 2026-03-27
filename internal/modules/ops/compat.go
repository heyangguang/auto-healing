package ops

import "github.com/company/auto-healing/internal/database"

func New() *Module {
	return NewWithDB(database.DB)
}
