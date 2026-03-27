package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// 初始化超级管理员脚本
func main() {
	cfg := mustLoadCLIConfig()
	initializeCLI(cfg)
	defer database.Close()

	ctx := context.Background()
	repos := newAdminRepos()

	ensureUsersTableEmpty(ctx, repos.user)
	admin, password, err := bootstrapInitialAdmin(ctx, repos)
	if err != nil {
		log.Fatalf("初始化平台管理员失败: %v", err)
	}
	printAdminBootstrapResult(admin, password)
	printPermissionCount(ctx, repos.permission)
}

func mustLoadCLIConfig() *config.Config {
	cfg, err := config.LoadRequired()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	return cfg
}

func initializeCLI(cfg *config.Config) {
	logger.Init(&cfg.Log)
	if err := database.Init(cfg); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	if err := prepareCLIData(); err != nil {
		log.Fatalf("初始化基础数据失败: %v", err)
	}
}

func prepareCLIData() error {
	jobs := []struct {
		name string
		run  func() error
	}{
		{name: "数据库迁移失败", run: database.AutoMigrate},
		{name: "权限种子同步失败", run: database.SyncPermissionsAndRoles},
		{name: "高危指令黑名单种子数据同步失败", run: database.SeedCommandBlacklist},
		{name: "平台设置默认值初始化失败", run: database.SeedPlatformSettings},
	}
	for _, job := range jobs {
		if err := job.run(); err != nil {
			return fmt.Errorf("%s: %w", job.name, err)
		}
	}
	return nil
}

type adminRepos struct {
	user       *accessrepo.UserRepository
	role       *accessrepo.RoleRepository
	permission *accessrepo.PermissionRepository
}

func newAdminRepos() adminRepos {
	return adminRepos{
		user:       accessrepo.NewUserRepositoryWithDB(database.DB),
		role:       accessrepo.NewRoleRepositoryWithDB(database.DB),
		permission: accessrepo.NewPermissionRepositoryWithDB(database.DB),
	}
}

func ensureUsersTableEmpty(ctx context.Context, userRepo *accessrepo.UserRepository) {
	count, err := userRepo.CountAll(ctx)
	if err != nil {
		log.Fatalf("查询用户数量失败: %v", err)
	}
	if count > 0 {
		fmt.Println("⚠️  数据库中已存在用户，跳过初始化")
		fmt.Println("💡 如需重新初始化，请先清空 users 表")
		os.Exit(0)
	}
}

func createInitialAdmin(ctx context.Context, userRepo *accessrepo.UserRepository) (accessmodel.User, string, error) {
	password, err := resolveInitialAdminPassword()
	if err != nil {
		return accessmodel.User{}, "", fmt.Errorf("生成初始密码失败: %w", err)
	}

	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		return accessmodel.User{}, "", fmt.Errorf("密码加密失败: %w", err)
	}

	admin := accessmodel.User{
		Username:        "admin",
		Email:           "admin@example.com",
		PasswordHash:    passwordHash,
		DisplayName:     "超级管理员",
		Status:          "active",
		IsPlatformAdmin: true,
	}

	if err := userRepo.Create(ctx, &admin); err != nil {
		return accessmodel.User{}, "", fmt.Errorf("创建用户失败: %w", err)
	}
	return admin, password, nil
}

func bootstrapInitialAdmin(ctx context.Context, repos adminRepos) (accessmodel.User, string, error) {
	admin, password, err := createInitialAdmin(ctx, repos.user)
	if err != nil {
		return accessmodel.User{}, "", err
	}
	if err := bindPlatformAdminRole(ctx, repos, admin.ID); err != nil {
		if deleteErr := repos.user.Delete(ctx, admin.ID); deleteErr != nil {
			return accessmodel.User{}, "", fmt.Errorf("绑定平台管理员角色失败: %w；回滚用户失败: %v", err, deleteErr)
		}
		return accessmodel.User{}, "", err
	}
	return admin, password, nil
}

func printAdminBootstrapResult(admin accessmodel.User, password string) {
	fmt.Println("✅ 平台管理员初始化成功!")
	fmt.Println("")
	fmt.Println("📝 登录信息:")
	fmt.Printf("   用户名: %s\n", admin.Username)
	fmt.Printf("   密码:   %s\n", password)
	fmt.Printf("   用户ID: %s\n", admin.ID)
	fmt.Println("")
	fmt.Println("⚠️  请尽快修改默认密码!")
	fmt.Println("")
}

func bindPlatformAdminRole(ctx context.Context, repos adminRepos, adminID uuid.UUID) error {
	err := bindPlatformAdminRoleWith(
		func(innerCtx context.Context, roleName string) (uuid.UUID, error) {
			role, lookupErr := repos.role.GetByName(innerCtx, roleName)
			if lookupErr != nil {
				return uuid.Nil, lookupErr
			}
			return role.ID, nil
		},
		func(innerCtx context.Context, userID uuid.UUID, roleIDs []uuid.UUID) error {
			return repos.user.AssignRoles(innerCtx, userID, roleIDs)
		},
		ctx,
		adminID,
	)
	if err != nil {
		return err
	}
	fmt.Println("🔑 已绑定 platform_admin 角色")
	return nil
}

func bindPlatformAdminRoleWith(
	getRoleID func(context.Context, string) (uuid.UUID, error),
	assignRole func(context.Context, uuid.UUID, []uuid.UUID) error,
	ctx context.Context,
	adminID uuid.UUID,
) error {
	roleID, err := getRoleID(ctx, "platform_admin")
	if err != nil {
		return fmt.Errorf("查询 platform_admin 角色失败: %w", err)
	}
	if err := assignRole(ctx, adminID, []uuid.UUID{roleID}); err != nil {
		return fmt.Errorf("写入用户角色关联失败: %w", err)
	}
	return nil
}

func printPermissionCount(ctx context.Context, permissionRepo *accessrepo.PermissionRepository) {
	permCount, err := permissionRepo.CountAll(ctx)
	if err != nil {
		log.Fatalf("查询权限数量失败: %v", err)
	}
	fmt.Printf("🔐 系统预置权限: %d 个\n", permCount)
	fmt.Println("")
}

func resolveInitialAdminPassword() (string, error) {
	password := strings.TrimSpace(os.Getenv("INIT_ADMIN_PASSWORD"))
	if password != "" {
		return password, nil
	}
	return crypto.GenerateRandomString(20)
}
