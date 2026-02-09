package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/handler"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/scheduler"
	"github.com/company/auto-healing/internal/service/healing"
	"github.com/gin-gonic/gin"
)

// @title Auto-Healing System API
// @version 1.0
// @description 运维自愈系统 API 文档
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	config.SetGlobalConfig(cfg)

	// 初始化日志
	logger.Init(&cfg.Log)
	defer logger.Sync()

	// 初始化数据库
	if err := database.Init(cfg); err != nil {
		logger.Fatal("数据库初始化失败: %v", err)
	}
	defer database.Close()

	// 同步预置权限和角色
	if err := database.SyncPermissionsAndRoles(); err != nil {
		logger.Error("权限种子同步失败: %v", err)
	}

	// 初始化 Redis
	if err := database.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 初始化失败: %v", err)
	}
	defer database.CloseRedis()

	// 启动插件同步调度器
	sched := scheduler.NewScheduler()
	sched.Start()
	defer sched.Stop()

	// 启动 Git 仓库同步调度器
	gitSched := scheduler.NewGitScheduler()
	gitSched.Start()
	defer gitSched.Stop()

	// 启动执行任务调度器
	execSched := scheduler.NewExecutionScheduler()
	execSched.Start()
	defer execSched.Stop()

	// 启动自愈调度器
	healingSched := healing.NewScheduler()
	healingSched.Start()
	defer healingSched.Stop()

	// 设置 Gin 模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建 Gin 引擎
	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 设置路由
	handler.SetupRoutes(r, cfg)

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("收到关闭信号，正在停止服务...")
		sched.Stop()
		gitSched.Stop()
		execSched.Stop()
		healingSched.Stop()
		os.Exit(0)
	}()

	// 启动服务
	addr := cfg.Server.Host + ":" + cfg.Server.Port
	logger.Info("启动服务于 %s", addr)
	if err := r.Run(addr); err != nil {
		logger.Error("服务启动失败: %v", err)
		os.Exit(1)
	}
}
