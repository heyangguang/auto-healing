package auth

import (
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/pkg/jwt"
)

func NewService(jwtSvc *jwt.Service) *Service {
	return NewServiceWithDB(database.DB, jwtSvc)
}
