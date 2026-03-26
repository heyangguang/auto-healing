//go:build integration

package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	integrationAdminDB   = "postgres"
	integrationDBPrefix  = "auto_healing_integration_"
	integrationDBUser    = "postgres"
	integrationDBPass    = "postgres"
	integrationDBHost    = "127.0.0.1"
	integrationDBPort    = "5432"
	integrationRedisHost = "127.0.0.1"
	integrationRedisPort = "6379"
	integrationJWTSecret = "integration-test-secret"
)

func TestLiveBootstrapAgainstPostgresAndRedis(t *testing.T) {
	dbName := integrationDBPrefix + fmt.Sprint(time.Now().UnixNano())
	adminDB := openAdminDB(t)
	createIntegrationDB(t, adminDB, dbName)
	t.Cleanup(func() {
		closeGormDB(t, adminDB)
		dropIntegrationDB(t, dbName)
	})

	cfg := integrationConfig(dbName)
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	if err := AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := SyncPermissionsAndRoles(); err != nil {
		t.Fatalf("SyncPermissionsAndRoles() error = %v", err)
	}
	if err := SeedPlatformSettings(); err != nil {
		t.Fatalf("SeedPlatformSettings() error = %v", err)
	}
	if err := SeedCommandBlacklist(); err != nil {
		t.Fatalf("SeedCommandBlacklist() error = %v", err)
	}
	if err := SeedSiteMessages(); err != nil {
		t.Fatalf("SeedSiteMessages() error = %v", err)
	}

	assertSeededCounts(t)

	if err := InitRedis(&cfg.Redis); err != nil {
		t.Fatalf("InitRedis() error = %v", err)
	}
	t.Cleanup(func() { _ = CloseRedis() })

	store := NewTokenBlacklistStore()
	exp := time.Now().Add(5 * time.Minute)
	if err := store.Add(t.Context(), "integration-jti", exp); err != nil {
		t.Fatalf("TokenBlacklistStore.Add() error = %v", err)
	}
	if !store.Exists(t.Context(), "integration-jti") {
		t.Fatal("TokenBlacklistStore.Exists() = false, want true")
	}
}

func integrationConfig(dbName string) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Name: "Auto-Healing",
			Env:  "development",
		},
		Database: config.DatabaseConfig{
			Host:               integrationDBHost,
			Port:               integrationDBPort,
			User:               integrationDBUser,
			Password:           integrationDBPass,
			DBName:             dbName,
			SSLMode:            "disable",
			MaxOpenConns:       5,
			MaxIdleConns:       2,
			MaxLifetimeMinutes: 5,
		},
		Redis: config.RedisConfig{
			Host: integrationRedisHost,
			Port: integrationRedisPort,
			DB:   9,
		},
		JWT: config.JWTConfig{
			Secret: integrationJWTSecret,
		},
		Log: config.LogConfig{
			Level: "error",
			Console: config.ConsoleLogConfig{
				Enabled: false,
			},
			File: config.FileLogConfig{
				Enabled: false,
			},
			DBLevel: "error",
		},
	}
}

func openAdminDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		integrationDBHost, integrationDBPort, integrationDBUser, integrationDBPass, integrationAdminDB,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open admin db error = %v", err)
	}
	return db
}

func createIntegrationDB(t *testing.T, adminDB *gorm.DB, dbName string) {
	t.Helper()

	if err := adminDB.Exec(`DROP DATABASE IF EXISTS "` + dbName + `"`).Error; err != nil {
		t.Fatalf("drop test db before create error = %v", err)
	}
	if err := adminDB.Exec(`CREATE DATABASE "` + dbName + `"`).Error; err != nil {
		t.Fatalf("create test db error = %v", err)
	}
}

func dropIntegrationDB(t *testing.T, dbName string) {
	t.Helper()

	adminDB := openAdminDB(t)
	defer closeGormDB(t, adminDB)

	if err := adminDB.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = ?`, dbName).Error; err != nil {
		t.Fatalf("terminate db connections error = %v", err)
	}
	if err := adminDB.Exec(`DROP DATABASE IF EXISTS "` + dbName + `"`).Error; err != nil {
		t.Fatalf("drop test db error = %v", err)
	}
}

func closeGormDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	_ = sqlDB.Close()
}

func assertSeededCounts(t *testing.T) {
	t.Helper()

	assertTableHasRows(t, "permissions")
	assertTableHasRows(t, "roles")
	assertTableHasRows(t, "platform_settings")
	assertTableHasRows(t, "command_blacklist")
	assertTableHasRows(t, "site_messages")
}

func assertTableHasRows(t *testing.T, table string) {
	t.Helper()

	var count int64
	if err := DB.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s error = %v", table, err)
	}
	if count == 0 {
		t.Fatalf("table %s count = 0, want > 0", table)
	}
}
