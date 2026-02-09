package database

import (
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB 全局数据库连接
var DB *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.Config) error {
	var err error

	// 根据配置设置 GORM 日志级别 (使用 log.db_level)
	// GORM 支持: info(显示所有SQL), warn(慢查询), error(只有错误), off(关闭)
	logLevel := gormlogger.Silent
	switch cfg.Log.DBLevel {
	case "info":
		logLevel = gormlogger.Info
	case "warn":
		logLevel = gormlogger.Warn
	case "error":
		logLevel = gormlogger.Error
	case "off":
		logLevel = gormlogger.Silent
	default:
		logLevel = gormlogger.Warn
	}

	DB, err = gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层 sql.DB 并配置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.MaxLifetime())

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	logger.Info("数据库连接成功")
	return nil
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate() error {
	logger.Info("开始自动迁移数据库表结构...")

	err := DB.AutoMigrate(
		// 用户权限
		&model.User{},
		&model.Role{},
		&model.Permission{},
		&model.UserRole{},
		&model.RolePermission{},
		&model.TokenBlacklist{},
		&model.RefreshToken{},
		// 插件
		&model.Plugin{},
		&model.PluginSyncLog{},
		&model.Incident{},
		// 工作流
		&model.Workflow{},
		&model.WorkflowNode{},
		&model.WorkflowEdge{},
		&model.WorkflowInstance{},
		&model.NodeExecution{},
		// 执行
		&model.GitRepository{},
		&model.Playbook{},
		&model.ExecutionTask{},
		// 通知
		&model.NotificationChannel{},
		&model.NotificationTemplate{},
		&model.NotificationLog{},
		// 日志
		&model.AuditLog{},
		&model.ExecutionLog{},
		&model.WorkflowLog{},
		// Dashboard
		&model.DashboardConfig{},
		&model.SystemWorkspace{},
		&model.RoleWorkspace{},
	)

	if err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	logger.Info("数据库表结构迁移完成")
	return nil
}

// Close 关闭数据库连接
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Transaction 事务执行
func Transaction(fn func(tx *gorm.DB) error) error {
	return DB.Transaction(fn)
}

// WithTimeout 带超时的数据库操作
func WithTimeout(timeout time.Duration) *gorm.DB {
	return DB.Session(&gorm.Session{
		NowFunc: func() time.Time {
			return time.Now()
		},
	})
}
