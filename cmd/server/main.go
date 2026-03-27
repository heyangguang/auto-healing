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
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	platformlifecycle "github.com/company/auto-healing/internal/platform/lifecycle"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

	db, cleanup := initializeInfrastructure(cfg)
	defer cleanup()
	defer middleware.Shutdown()
	if err := runStartupJobsWithDeps(signalCtx, newStartupDepsWithDB(db)); err != nil {
		logger.Fatal("启动任务失败: %v", err)
	}
	schedulers := startSchedulersWithDB(db)
	defer stopSchedulers(schedulers)

	r := newRouterWithDB(cfg, db)
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

func initializeInfrastructure(cfg *config.Config) (*gorm.DB, func()) {
	logger.Init(&cfg.Log)
	db, err := database.Init(cfg)
	if err != nil {
		logger.Fatal("数据库初始化失败: %v", err)
	}
	if err := database.AutoMigrate(); err != nil {
		logger.Fatal("数据库迁移失败: %v", err)
	}
	if err := database.InitRedis(&cfg.Redis); err != nil {
		logger.Fatal("Redis 初始化失败: %v", err)
	}
	return db, func() {
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

type startupDeps struct {
	dictionary dictionarySeeder
	cleaner    siteMessageCleaner
}

var listStartupSeedJobs = startupSeedJobs

func newStartupDepsWithDB(db *gorm.DB) startupDeps {
	settingsRepo := settingsrepo.NewPlatformSettingsRepositoryWithDB(db)
	return startupDeps{
		dictionary: opsservice.NewDictionaryServiceWithDeps(opsservice.DictionaryServiceDeps{
			Repo: opsrepo.NewDictionaryRepositoryWithDB(db),
		}),
		cleaner: engagementrepo.NewSiteMessageRepositoryWithDeps(engagementrepo.SiteMessageRepositoryDeps{
			DB:               db,
			PlatformSettings: settingsRepo,
		}),
	}
}

func runStartupJobsWithDeps(ctx context.Context, deps startupDeps) error {
	for _, job := range listStartupSeedJobs() {
		if err := runStartupJob(job); err != nil {
			return err
		}
	}

	if err := deps.dictionary.SeedDictionaries(ctx); err != nil {
		return fmt.Errorf("字典种子数据同步失败: %w", err)
	}

	if _, err := deps.cleaner.CleanExpired(ctx); err != nil {
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

func startSchedulersWithDB(db *gorm.DB) []lifecycleService {
	db = requireServerDB(db, "server schedulers")
	schedulers := []lifecycleService{
		appruntime.NewManagerWithDeps(appruntime.ManagerDeps{DB: db}),
		healing.NewSchedulerWithDB(db),
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

func newRouterWithDB(cfg *config.Config, db *gorm.DB) *gin.Engine {
	db = requireServerDB(db, "server router")
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.GET("/health", func(c *gin.Context) {
		response.Success(c, healthStatusResponse{Status: "ok"})
	})
	httproutes.SetupRoutesWithDB(r, cfg, db)
	middleware.ValidateAuditResourceTypes(r)
	return r
}

func requireServerDB(db *gorm.DB, component string) *gorm.DB {
	if db == nil {
		panic(component + " requires explicit db")
	}
	return db
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
