package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/company/auto-healing/internal/config"
	accessmodel "github.com/company/auto-healing/internal/modules/access/model"
	automodel "github.com/company/auto-healing/internal/modules/automation/model"
	engagementmodel "github.com/company/auto-healing/internal/modules/engagement/model"
	integrationsmodel "github.com/company/auto-healing/internal/modules/integrations/model"
	opsmodel "github.com/company/auto-healing/internal/modules/ops/model"
	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB 全局数据库连接
var DB *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.Config) (*gorm.DB, error) {
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

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, // 迁移时不创建/修改外键约束，避免关联表冲突
		Logger: gormlogger.New(
			log.New(os.Stderr, "\r\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层 sql.DB 并配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.MaxLifetime())

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	DB = db
	logger.Info("数据库连接成功")
	return db, nil
}

// AutoMigrate 自动迁移数据库表结构（增量：只创建不存在的表）
func AutoMigrate() error {
	logger.Info("开始自动迁移数据库表结构...")
	migrated, err := migrateMissingTables(DB, autoMigrateModels())
	if err != nil {
		return err
	}
	logAutoMigrateResult(migrated)

	if err := ensureRoleWorkspaceSchema(); err != nil {
		return fmt.Errorf("修正 role_workspaces 表结构失败: %w", err)
	}
	if err := ensureSystemWorkspaceSchema(); err != nil {
		return fmt.Errorf("修正 system_workspaces 表结构失败: %w", err)
	}
	return nil
}

func autoMigrateModels() []interface{} {
	return []interface{}{
		&accessmodel.User{}, &accessmodel.Role{}, &accessmodel.Permission{}, &accessmodel.UserPlatformRole{}, &accessmodel.RolePermission{}, &accessmodel.TokenBlacklist{}, &accessmodel.RefreshToken{},
		&integrationsmodel.Plugin{}, &integrationsmodel.PluginSyncLog{}, &platformmodel.Incident{},
		&automodel.Workflow{}, &automodel.WorkflowNode{}, &automodel.WorkflowEdge{}, &automodel.WorkflowInstance{}, &automodel.NodeExecution{},
		&integrationsmodel.GitRepository{}, &integrationsmodel.GitSyncLog{}, &integrationsmodel.Playbook{}, &integrationsmodel.PlaybookScanLog{}, &automodel.ExecutionTask{}, &automodel.ExecutionRun{}, &automodel.ExecutionSchedule{},
		&engagementmodel.NotificationChannel{}, &engagementmodel.NotificationTemplate{}, &engagementmodel.NotificationLog{},
		&platformmodel.AuditLog{}, &platformmodel.PlatformAuditLog{}, &automodel.ExecutionLog{}, &automodel.WorkflowLog{},
		&engagementmodel.DashboardConfig{}, &engagementmodel.SystemWorkspace{}, &engagementmodel.RoleWorkspace{},
		&engagementmodel.UserPreference{}, &engagementmodel.UserFavorite{}, &engagementmodel.UserRecent{},
		&automodel.HealingFlow{}, &automodel.HealingRule{}, &automodel.FlowInstance{}, &automodel.ApprovalTask{}, &automodel.FlowExecutionLog{},
		&engagementmodel.SiteMessage{}, &engagementmodel.SiteMessageRead{}, &opsmodel.PlatformSetting{},
		&accessmodel.Tenant{}, &accessmodel.UserTenantRole{}, &opsmodel.Dictionary{}, &accessmodel.TenantInvitation{},
		&opsmodel.CommandBlacklist{}, &opsmodel.BlacklistExemption{}, &opsmodel.TenantBlacklistOverride{},
		&platformmodel.CMDBItem{}, &platformmodel.CMDBMaintenanceLog{},
		&accessmodel.ImpersonationRequest{}, &accessmodel.ImpersonationApprover{},
		&secretsmodel.SecretsSource{},
	}
}

func migrateMissingTables(db *gorm.DB, models []interface{}) (int, error) {
	migrated := 0
	for _, current := range models {
		created, err := migrateModelIfMissing(db, current)
		if err != nil {
			return 0, err
		}
		if created {
			migrated++
		}
	}
	return migrated, nil
}

func migrateModelIfMissing(db *gorm.DB, current interface{}) (bool, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(current); err != nil {
		return false, fmt.Errorf("解析模型 %T 失败: %w", current, err)
	}
	tableName := stmt.Schema.Table
	if db.Migrator().HasTable(tableName) {
		return false, nil
	}
	if err := db.AutoMigrate(current); err != nil {
		return false, fmt.Errorf("迁移表 %s 失败: %w", tableName, err)
	}
	logger.Info("已创建表: %s", tableName)
	return true, nil
}

func logAutoMigrateResult(migrated int) {
	if migrated > 0 {
		logger.Info("数据库迁移完成，新建 %d 张表", migrated)
		return
	}
	logger.Info("数据库表结构已是最新，无需迁移")
}

func ensureRoleWorkspaceSchema() error {
	statements := []string{
		`ALTER TABLE role_workspaces ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid()`,
		`ALTER TABLE role_workspaces ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id)`,
		`ALTER TABLE role_workspaces ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW()`,
		`CREATE INDEX IF NOT EXISTS idx_role_workspaces_tenant_id ON role_workspaces(tenant_id)`,
	}

	for _, sql := range statements {
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureSystemWorkspaceSchema() error {
	statements := []string{
		`ALTER TABLE system_workspaces ADD COLUMN IF NOT EXISTS is_readonly BOOLEAN NOT NULL DEFAULT FALSE`,
	}

	for _, sql := range statements {
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedPlatformSettings 初始化平台设置默认值（幂等，使用 ON CONFLICT DO NOTHING）
func SeedPlatformSettings() error {
	type row struct {
		Key          string
		Value        string
		Type         string
		Module       string
		Label        string
		Description  string
		DefaultValue string
	}

	defaults := []row{
		// 邮件发送（SMTP）
		{"email.smtp_host", "", "string", "email", "SMTP 服务器地址", "邮件发送服务器地址，如 smtp.example.com", ""},
		{"email.smtp_port", "587", "int", "email", "SMTP 端口", "常用：587（STARTTLS）、465（SSL）、25（明文）", "587"},
		{"email.username", "", "string", "email", "SMTP 账号", "SMTP 登录用户名", ""},
		{"email.password", "", "string", "email", "SMTP 密码", "SMTP 登录密码", ""},
		{"email.from_address", "", "string", "email", "发件人地址", "邮件发件人地址，如 no-reply@example.com", ""},
		{"email.use_tls", "true", "bool", "email", "启用 TLS", "是否使用 TLS/SSL 加密连接（推荐开启）", "true"},
		// 邀请
		{"email.invitation_expire_days", "7", "int", "email", "邀请链接有效期（天）", "租户邀请链接的有效天数，超期自动失效", "7"},
		// 站点
		{"site.base_url", "", "string", "site", "站点访问地址", "平台对外访问的根地址，用于生成邀请链接等，如 https://example.com", ""},
		// 站内信
		{"site_message.retention_days", "90", "int", "site_message", "站内信保留天数", "站内消息超过该天数后将被自动清理", "90"},
	}

	for _, d := range defaults {
		sql := `INSERT INTO platform_settings (key, value, type, module, label, description, default_value, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
			ON CONFLICT (key) DO NOTHING`
		if err := DB.Exec(sql, d.Key, d.Value, d.Type, d.Module, d.Label, d.Description, d.DefaultValue).Error; err != nil {
			return fmt.Errorf("初始化平台设置 %s 失败: %w", d.Key, err)
		}
	}
	logger.Info("平台设置默认值初始化完成（%d 项）", len(defaults))
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
