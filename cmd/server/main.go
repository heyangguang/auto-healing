package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httproutes "github.com/company/auto-healing/internal/app/httpapi"
	appruntime "github.com/company/auto-healing/internal/app/runtime"
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/modules/automation/service/healing"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/logger"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
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
	if err := runStartupJobs(signalCtx); err != nil {
		logger.Fatal("启动任务失败: %v", err)
	}
	schedulers := startSchedulers()
	defer stopSchedulers(schedulers)

	r := newRouter(cfg)
	defer platformlifecycle.Cleanup()
	server := newHTTPServer(cfg, r)
	logger.Info("启动服务于 %s", server.Addr)
	if err := runHTTPServer(signalCtx, server, 10*time.Second, logShutdownSignal); err != nil {
		logger.Error("服务启动失败: %v", err)
		os.Exit(1)
	}
}

func mustLoadConfig() *config.Config {
	cfg, err := config.LoadRequired()
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

type startupJob struct {
	name     string
	required bool
	run      func() error
}

type dictionarySeeder interface {
	SeedDictionaries(context.Context) error
}

type siteMessageCleaner interface {
	CleanExpired(context.Context) (int64, error)
}

var (
	newDictionaryService = func() dictionarySeeder {
		return opsservice.NewDictionaryService()
	}
	newSiteMessageRepo = func() siteMessageCleaner {
		return engagementrepo.NewSiteMessageRepository()
	}
	listStartupSeedJobs = startupSeedJobs
)

func runStartupJobs(ctx context.Context) error {
	for _, job := range listStartupSeedJobs() {
		if err := runStartupJob(job); err != nil {
			return err
		}
	}

	dictSvc := newDictionaryService()
	if err := dictSvc.SeedDictionaries(ctx); err != nil {
		return fmt.Errorf("字典种子数据同步失败: %w", err)
	}

	siteMessageRepo := newSiteMessageRepo()
	if _, err := siteMessageRepo.CleanExpired(ctx); err != nil {
		logger.Error("站内信过期清理失败: %v", err)
	}
	return nil
}

func startupSeedJobs() []startupJob {
	return []startupJob{
		{name: "权限种子同步失败", required: true, run: database.SyncPermissionsAndRoles},
		{name: "站内信种子数据插入失败", required: false, run: database.SeedSiteMessages},
		{name: "高危指令黑名单种子数据同步失败", required: true, run: database.SeedCommandBlacklist},
		{name: "平台设置默认值初始化失败", required: true, run: database.SeedPlatformSettings},
	}
}

func runStartupJob(job startupJob) error {
	if err := job.run(); err != nil {
		if job.required {
			return fmt.Errorf("%s: %w", job.name, err)
		}
		logger.Error("%s: %v", job.name, err)
	}
	return nil
}

type lifecycleService interface {
	Start()
	Stop()
}

func startSchedulers() []lifecycleService {
	schedulers := []lifecycleService{
		appruntime.NewScheduler(),
		appruntime.NewGitScheduler(),
		appruntime.NewExecutionScheduler(),
		appruntime.NewNotificationRetryScheduler(),
		appruntime.NewBlacklistExemptionScheduler(),
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
	httproutes.SetupRoutes(r, cfg)
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
