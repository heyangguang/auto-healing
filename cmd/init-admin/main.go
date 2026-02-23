package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 初始化超级管理员脚本
func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	if err := database.Init(cfg); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	// 检查是否已有超级管理员
	var count int64
	database.DB.Model(&model.User{}).Count(&count)
	if count > 0 {
		fmt.Println("⚠️  数据库中已存在用户，跳过初始化")
		fmt.Println("💡 如需重新初始化，请先清空 users 表")
		os.Exit(0)
	}

	// 获取超级管理员角色
	var superAdminRole model.Role
	if err := database.DB.Where("name = ?", "super_admin").First(&superAdminRole).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Fatalf("未找到 super_admin 角色，请先执行数据库迁移脚本")
		}
		log.Fatalf("查询角色失败: %v", err)
	}

	// 创建超级管理员用户
	password := "admin123456"
	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		log.Fatalf("密码加密失败: %v", err)
	}

	admin := model.User{
		Username:     "admin",
		Email:        "admin@example.com",
		PasswordHash: passwordHash,
		DisplayName:  "超级管理员",
		Status:       "active",
	}

	if err := database.DB.Create(&admin).Error; err != nil {
		log.Fatalf("创建用户失败: %v", err)
	}

	// 分配超级管理员角色
	userRole := model.UserTenantRole{
		UserID:   admin.ID,
		TenantID: model.DefaultTenantID,
		RoleID:   superAdminRole.ID,
	}
	if err := database.DB.Create(&userRole).Error; err != nil {
		log.Fatalf("分配角色失败: %v", err)
	}

	fmt.Println("✅ 超级管理员初始化成功!")
	fmt.Println("")
	fmt.Println("📝 登录信息:")
	fmt.Printf("   用户名: %s\n", admin.Username)
	fmt.Printf("   密码: %s\n", password)
	fmt.Printf("   用户ID: %s\n", admin.ID)
	fmt.Println("")
	fmt.Println("⚠️  请尽快修改默认密码!")

	// 显示角色信息
	fmt.Println("")
	fmt.Println("📊 系统预置角色:")
	var roles []model.Role
	database.DB.Find(&roles)
	for _, r := range roles {
		fmt.Printf("   - %s (%s)\n", r.DisplayName, r.Name)
	}

	// 显示权限数量
	var permCount int64
	database.DB.Model(&model.Permission{}).Count(&permCount)
	fmt.Println("")
	fmt.Printf("🔐 系统预置权限: %d 个\n", permCount)

	_ = ctx
	_ = uuid.Nil
}
