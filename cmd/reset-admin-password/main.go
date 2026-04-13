package main

import (
	"context"
	"errors"
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
	"gorm.io/gorm"
)

const (
	defaultAdminUsername    = "admin"
	generatedPasswordLength = 20
)

var ErrTargetNotPlatformAdmin = errors.New("目标用户不是平台管理员")

func main() {
	cfg := mustLoadCLIConfig()
	db := initializeCLI(cfg)
	defer database.Close()
	defer logger.Sync()

	username := resolveTargetAdminUsername()
	password, err := resolveResetAdminPassword()
	if err != nil {
		log.Fatalf("生成新密码失败: %v", err)
	}

	user, err := resetPlatformAdminPassword(
		context.Background(),
		accessrepo.NewUserRepositoryWithDB(db),
		username,
		password,
	)
	if err != nil {
		log.Fatalf("重置平台管理员密码失败: %v", err)
	}

	printResetResult(user, password)
}

func mustLoadCLIConfig() *config.Config {
	cfg, err := config.LoadRequired()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	return cfg
}

func initializeCLI(cfg *config.Config) *gorm.DB {
	logger.Init(&cfg.Log)
	db, err := database.Init(cfg)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	return db
}

func resetPlatformAdminPassword(
	ctx context.Context,
	userRepo *accessrepo.UserRepository,
	username string,
	password string,
) (accessmodel.User, error) {
	return resetPlatformAdminPasswordWith(
		userRepo.GetByUsername,
		userRepo.UpdatePassword,
		crypto.HashPassword,
		ctx,
		username,
		password,
	)
}

func resetPlatformAdminPasswordWith(
	getUser func(context.Context, string) (*accessmodel.User, error),
	updatePassword func(context.Context, uuid.UUID, string) error,
	hashPassword func(string) (string, error),
	ctx context.Context,
	username string,
	password string,
) (accessmodel.User, error) {
	user, err := getUser(ctx, username)
	if err != nil {
		return accessmodel.User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	if err := validatePlatformAdmin(*user); err != nil {
		return accessmodel.User{}, err
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return accessmodel.User{}, fmt.Errorf("密码加密失败: %w", err)
	}
	if err := updatePassword(ctx, user.ID, passwordHash); err != nil {
		return accessmodel.User{}, fmt.Errorf("更新密码失败: %w", err)
	}
	return *user, nil
}

func validatePlatformAdmin(user accessmodel.User) error {
	if !user.IsPlatformAdmin {
		return ErrTargetNotPlatformAdmin
	}
	return nil
}

func printResetResult(user accessmodel.User, password string) {
	fmt.Println("✅ 平台管理员密码重置成功!")
	fmt.Println("")
	fmt.Println("📝 登录信息:")
	fmt.Printf("   用户名: %s\n", user.Username)
	fmt.Printf("   密码:   %s\n", password)
	fmt.Printf("   用户ID: %s\n", user.ID)
	fmt.Println("")
}

func resolveTargetAdminUsername() string {
	username := strings.TrimSpace(os.Getenv("RESET_ADMIN_USERNAME"))
	if username == "" {
		return defaultAdminUsername
	}
	return username
}

func resolveResetAdminPassword() (string, error) {
	password := strings.TrimSpace(os.Getenv("RESET_ADMIN_PASSWORD"))
	if password != "" {
		return password, nil
	}
	return crypto.GenerateRandomString(generatedPasswordLength)
}
