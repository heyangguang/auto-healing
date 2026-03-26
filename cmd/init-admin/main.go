package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
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
	admin, password := createInitialAdmin(ctx, repos.user)
	printAdminBootstrapResult(admin, password)
	bindPlatformAdminRole(ctx, repos, admin.ID)
	printPermissionCount(ctx, repos.permission)
}

func mustLoadCLIConfig() *config.Config {
	cfg, err := config.Load()
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
}

type adminRepos struct {
	user       *repository.UserRepository
	role       *repository.RoleRepository
	permission *repository.PermissionRepository
}

func newAdminRepos() adminRepos {
	return adminRepos{
		user:       repository.NewUserRepository(),
		role:       repository.NewRoleRepository(),
		permission: repository.NewPermissionRepository(),
	}
}

func ensureUsersTableEmpty(ctx context.Context, userRepo *repository.UserRepository) {
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

func createInitialAdmin(ctx context.Context, userRepo *repository.UserRepository) (model.User, string) {
	password, err := resolveInitialAdminPassword()
	if err != nil {
		log.Fatalf("生成初始密码失败: %v", err)
	}

	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		log.Fatalf("密码加密失败: %v", err)
	}

	admin := model.User{
		Username:        "admin",
		Email:           "admin@example.com",
		PasswordHash:    passwordHash,
		DisplayName:     "超级管理员",
		Status:          "active",
		IsPlatformAdmin: true,
	}

	if err := userRepo.Create(ctx, &admin); err != nil {
		log.Fatalf("创建用户失败: %v", err)
	}
	return admin, password
}

func printAdminBootstrapResult(admin model.User, password string) {
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

func bindPlatformAdminRole(ctx context.Context, repos adminRepos, adminID uuid.UUID) {
	platformAdminRole, err := repos.role.GetByName(ctx, "platform_admin")
	if err == nil {
		if err := repos.user.AssignRoles(ctx, adminID, []uuid.UUID{platformAdminRole.ID}); err != nil {
			fmt.Printf("⚠️  角色绑定失败: %v（不影响使用，admin 仍为 IsPlatformAdmin）\n", err)
		} else {
			fmt.Println("🔑 已绑定 platform_admin 角色")
		}
		return
	}
	fmt.Println("⚠️  platform_admin 角色未找到，请先启动一次 server 以初始化角色种子数据")
}

func printPermissionCount(ctx context.Context, permissionRepo *repository.PermissionRepository) {
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
