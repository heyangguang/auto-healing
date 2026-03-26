package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/handler"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/company/auto-healing/internal/scheduler"
	"github.com/company/auto-healing/internal/service"
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
	cfg := mustLoadConfig()
	config.SetGlobalConfig(cfg)
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cleanup := initializeInfrastructure(cfg)
	defer cleanup()
	defer middleware.Shutdown()
	runStartupJobs(signalCtx)
	schedulers := startSchedulers()
	defer stopSchedulers(schedulers)

	r := newRouter(cfg)
	defer handler.Cleanup()
	server := newHTTPServer(cfg, r)
	logger.Info("启动服务于 %s", server.Addr)
	if err := runHTTPServer(signalCtx, server, 10*time.Second, logShutdownSignal); err != nil {
		logger.Error("服务启动失败: %v", err)
		os.Exit(1)
	}
}

func mustLoadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	return cfg
}

func initializeInfrastructure(cfg *config.Config) func() {
	logger.Init(&cfg.Log)
	if err := database.Init(cfg); err != nil {
		logger.Fatal("数据库初始化失败: %v", err)
	}
	if err := database.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败: %v", err)
	}
	if err := database.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 初始化失败: %v", err)
	}
	return func() {
		database.CloseRedis()
		database.Close()
		logger.Sync()
	}
}

func runStartupJobs(ctx context.Context) {
	runSeedJob("权限种子同步失败", database.SyncPermissionsAndRoles)
	runSeedJob("站内信种子数据插入失败", database.SeedSiteMessages)
	runSeedJob("高危指令黑名单种子数据同步失败", database.SeedCommandBlacklist)
	runSeedJob("平台设置默认值初始化失败", database.SeedPlatformSettings)

	dictSvc := service.NewDictionaryService()
	if err := dictSvc.SeedDictionaries(ctx); err != nil {
		logger.Error("字典种子数据同步失败: %v", err)
	}

	siteMessageRepo := repository.NewSiteMessageRepository()
	if _, err := siteMessageRepo.CleanExpired(ctx); err != nil {
		logger.Error("站内信过期清理失败: %v", err)
	}
}

func runSeedJob(message string, fn func() error) {
	if err := fn(); err != nil {
		logger.Error("%s: %v", message, err)
	}
}

type lifecycleService interface {
	Start()
	Stop()
}

func startSchedulers() []lifecycleService {
	schedulers := []lifecycleService{
		scheduler.NewScheduler(),
		scheduler.NewGitScheduler(),
		scheduler.NewExecutionScheduler(),
		scheduler.NewNotificationRetryScheduler(),
		scheduler.NewBlacklistExemptionScheduler(),
		healing.DefaultScheduler(),
	}
	for _, item := range schedulers {
		item.Start()
	}
	return schedulers
}

func stopSchedulers(schedulers []lifecycleService) {
	for i := len(schedulers) - 1; i >= 0; i-- {
		schedulers[i].Stop()
	}
}

func newRouter(cfg *config.Config) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	handler.SetupRoutes(r, cfg)
	middleware.ValidateAuditResourceTypes(r)
	return r
}

func newHTTPServer(cfg *config.Config, router http.Handler) *http.Server {
	return &http.Server{
		Addr:    cfg.Server.Host + ":" + cfg.Server.Port,
		Handler: router,
	}
}

func logShutdownSignal() {
	logger.Info("收到关闭信号，正在停止服务...")
}

func runHTTPServer(ctx context.Context, server *http.Server, shutdownTimeout time.Duration, onShutdown func()) error {
	errCh := make(chan error, 1)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		if onShutdown != nil {
			onShutdown()
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return <-errCh
	}
}
