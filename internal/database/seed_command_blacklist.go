package database

import (
	"context"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

// SeedCommandBlacklist 种子化高危指令黑名单（增量：只插入不存在的）
func SeedCommandBlacklist() error {
	ctx := context.Background()

	seeds := []model.CommandBlacklist{
		// ========== filesystem ==========
		{
			Name:        "删除根目录",
			Pattern:     "rm -rf /",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "filesystem",
			Description: "递归强制删除根目录，会导致系统完全不可用",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "删除全盘文件",
			Pattern:     "rm -rf /*",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "filesystem",
			Description: "递归删除根目录下所有文件",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "格式化磁盘",
			Pattern:     "mkfs",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "filesystem",
			Description: "格式化磁盘分区，会导致数据丢失",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "DD 写磁盘",
			Pattern:     `dd\s+if=.*\s+of=/dev/`,
			MatchType:   "regex",
			Severity:    "critical",
			Category:    "filesystem",
			Description: "直接写入磁盘设备，可能覆盖系统或数据分区",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "清空磁盘设备",
			Pattern:     "> /dev/sda",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "filesystem",
			Description: "将空内容重定向到磁盘设备",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "Chmod 777 根目录",
			Pattern:     "chmod -R 777 /",
			MatchType:   "contains",
			Severity:    "high",
			Category:    "filesystem",
			Description: "递归给根目录所有文件最大权限，严重安全隐患",
			IsActive:    false,
			IsSystem:    true,
		},
		// ========== system ==========
		{
			Name:        "关机命令",
			Pattern:     "shutdown",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "system",
			Description: "关闭系统",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "重启命令",
			Pattern:     "reboot",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "system",
			Description: "重启系统",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "Fork 炸弹",
			Pattern:     ":(){ :|: & };:",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "system",
			Description: "无限递归创建进程导致系统资源耗尽",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "读取密码文件",
			Pattern:     "cat /etc/shadow",
			MatchType:   "contains",
			Severity:    "high",
			Category:    "system",
			Description: "读取系统密码哈希文件",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "初始化 init 0",
			Pattern:     "init 0",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "system",
			Description: "将系统运行级别切换到 0（关机）",
			IsActive:    false,
			IsSystem:    true,
		},
		// ========== network ==========
		{
			Name:        "清空防火墙规则",
			Pattern:     "iptables -F",
			MatchType:   "contains",
			Severity:    "high",
			Category:    "network",
			Description: "清空所有防火墙规则，可能暴露服务端口",
			IsActive:    false,
			IsSystem:    true,
		},
		// ========== database ==========
		{
			Name:        "删除数据库",
			Pattern:     "DROP DATABASE",
			MatchType:   "contains",
			Severity:    "critical",
			Category:    "database",
			Description: "删除整个数据库",
			IsActive:    false,
			IsSystem:    true,
		},
		{
			Name:        "删除数据表",
			Pattern:     "DROP TABLE",
			MatchType:   "contains",
			Severity:    "high",
			Category:    "database",
			Description: "删除数据库表",
			IsActive:    false,
			IsSystem:    true,
		},
		// ========== docker ==========
		{
			Name:        "强制删除所有容器",
			Pattern:     `docker\s+rm\s+-f\s+\$\(docker\s+ps`,
			MatchType:   "regex",
			Severity:    "critical",
			Category:    "system",
			Description: "批量强制删除所有 Docker 容器",
			IsActive:    false,
			IsSystem:    true,
		},
	}

	now := time.Now()
	inserted := 0

	for _, seed := range seeds {
		// 检查是否已存在（按 name + pattern 判断）
		var count int64
		DB.WithContext(ctx).Model(&model.CommandBlacklist{}).
			Where("name = ? AND pattern = ?", seed.Name, seed.Pattern).
			Count(&count)

		if count == 0 {
			seed.ID = uuid.New()
			seed.CreatedAt = now
			seed.UpdatedAt = now
			if err := DB.WithContext(ctx).Create(&seed).Error; err != nil {
				logger.Warn("插入黑名单种子数据失败: %s (%v)", seed.Name, err)
				continue
			}
			inserted++
		}
	}

	if inserted > 0 {
		logger.Info("高危指令黑名单种子数据: 新增 %d 条", inserted)
	}
	return nil
}
