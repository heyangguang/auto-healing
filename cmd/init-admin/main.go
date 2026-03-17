package main

import (
	"fmt"
	"log"
	"os"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/company/auto-healing/internal/pkg/logger"
)

// 初始化超级管理员脚本
func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化 logger（必须在 database.Init 之前，否则 logger.Info 会 panic）
	logger.Init(&cfg.Log)

	// 初始化数据库
	if err := database.Init(cfg); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Close()

	// 检查是否已有用户
	var count int64
	database.DB.Model(&model.User{}).Count(&count)
	if count > 0 {
		fmt.Println("⚠️  数据库中已存在用户，跳过初始化")
		fmt.Println("💡 如需重新初始化，请先清空 users 表")
		os.Exit(0)
	}

	// 创建平台管理员用户
	// IsPlatformAdmin=true 即拥有平台级全部权限，无需绑定租户角色
	password := "admin123456"
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

	if err := database.DB.Create(&admin).Error; err != nil {
		log.Fatalf("创建用户失败: %v", err)
	}

	fmt.Println("✅ 平台管理员初始化成功!")
	fmt.Println("")
	fmt.Println("📝 登录信息:")
	fmt.Printf("   用户名: %s\n", admin.Username)
	fmt.Printf("   密码:   %s\n", password)
	fmt.Printf("   用户ID: %s\n", admin.ID)
	fmt.Println("")
	fmt.Println("⚠️  请尽快修改默认密码!")
	fmt.Println("")

	// 绑定 platform_admin 角色
	var platformAdminRole model.Role
	if err := database.DB.Where("name = ?", "platform_admin").First(&platformAdminRole).Error; err == nil {
		userRole := model.UserPlatformRole{
			UserID: admin.ID,
			RoleID: platformAdminRole.ID,
		}
		if err := database.DB.Create(&userRole).Error; err != nil {
			fmt.Printf("⚠️  角色绑定失败: %v（不影响使用，admin 仍为 IsPlatformAdmin）\n", err)
		} else {
			fmt.Println("🔑 已绑定 platform_admin 角色")
		}
	} else {
		fmt.Println("⚠️  platform_admin 角色未找到，请先启动一次 server 以初始化角色种子数据")
	}

	// 显示权限数量
	var permCount int64
	database.DB.Model(&model.Permission{}).Count(&permCount)
	fmt.Printf("🔐 系统预置权限: %d 个\n", permCount)
	fmt.Println("")
}
