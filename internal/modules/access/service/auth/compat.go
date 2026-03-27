package auth

import (
	"github.com/company/auto-healing/internal/database"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/jwt"
	"gorm.io/gorm"
)

func NewService(jwtSvc *jwt.Service) *Service {
	return NewServiceWithDB(database.DB, jwtSvc)
}

func NewServiceWithDB(db *gorm.DB, jwtSvc *jwt.Service) *Service {
	return NewServiceWithDeps(DefaultServiceDepsWithDB(db, jwtSvc))
}

func DefaultServiceDepsWithDB(db *gorm.DB, jwtSvc *jwt.Service) ServiceDeps {
	return ServiceDeps{
		UserRepo:       accessrepo.NewUserRepositoryWithDB(db),
		RoleRepo:       accessrepo.NewRoleRepositoryWithDB(db),
		PermissionRepo: accessrepo.NewPermissionRepositoryWithDB(db),
		TenantRepo:     accessrepo.NewTenantRepositoryWithDB(db),
		JWTService:     jwtSvc,
		DB:             db,
	}
}
