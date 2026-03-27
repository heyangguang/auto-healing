package service

import (
	"context"
	"testing"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/ops/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDictionaryServiceGetAllReturnsClonedCache(t *testing.T) {
	svc := &DictionaryService{
		cache: map[string][]model.Dictionary{
			"status": {{
				DictType: "status",
				DictKey:  "ok",
				Label:    "OK",
				Extra:    model.JSON{"nested": map[string]interface{}{"level": "safe"}},
			}},
		},
	}

	data, err := svc.GetAll(context.Background(), nil, true)
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	data["status"][0].Label = "changed"
	extra := data["status"][0].Extra["nested"].(map[string]interface{})
	extra["level"] = "mutated"

	if svc.cache["status"][0].Label != "OK" {
		t.Fatalf("cache label = %q, want %q", svc.cache["status"][0].Label, "OK")
	}
	cachedExtra := svc.cache["status"][0].Extra["nested"].(map[string]interface{})
	if cachedExtra["level"] != "safe" {
		t.Fatalf("cache extra level = %v, want safe", cachedExtra["level"])
	}
}

func TestDictionaryServiceLoadCacheReturnsErrorAndBlocksFallback(t *testing.T) {
	db := openDictionaryTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })
	logger.Init(&config.LogConfig{})

	svc := NewDictionaryService()
	err := svc.LoadCache(context.Background())
	if err == nil {
		t.Fatal("LoadCache() error = nil, want missing table error")
	}

	_, err = svc.GetAll(context.Background(), nil, true)
	if err == nil {
		t.Fatal("GetAll() error = nil, want cached load error")
	}
}

func openDictionaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:dictionary-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
