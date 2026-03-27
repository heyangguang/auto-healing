package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/engagement/model"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	opsmodel "github.com/company/auto-healing/internal/modules/ops/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	respPkg "github.com/company/auto-healing/internal/pkg/response"
	platformevents "github.com/company/auto-healing/internal/platform/events"
	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSiteMessageHandlerGetCategoriesUsesDictionaryItems(t *testing.T) {
	db := openSiteMessageHandlerTestDB(t)
	seedSiteMessageDictionaryItem(t, db, "site_message_category", "service_notice", "服务通知", 0, true)
	seedSiteMessageDictionaryItem(t, db, "site_message_category", "announcement", "系统公告", 1, true)

	handler := newSiteMessageHandlerForTest(db)
	router := gin.New()
	router.GET("/categories", handler.GetCategories)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	resp := decodeSiteMessageResponse(t, recorder)
	items := decodeSiteMessageCategoryItems(t, resp.Data)
	if len(items) != 2 {
		t.Fatalf("expected 2 active categories, got %d", len(items))
	}
	if items[0].Value != "service_notice" || items[0].Label != "服务通知" {
		t.Fatalf("unexpected first category: %+v", items[0])
	}
	if items[1].Value != "announcement" || items[1].Label != "系统公告" {
		t.Fatalf("unexpected second category: %+v", items[1])
	}
}

func TestSiteMessageHandlerCreateMessageRejectsUnknownCategory(t *testing.T) {
	db := openSiteMessageHandlerTestDB(t)
	seedSiteMessageDictionaryItem(t, db, "site_message_category", "service_notice", "服务通知", 0, true)

	handler := newSiteMessageHandlerForTest(db)
	router := gin.New()
	router.POST("/messages", handler.CreateMessage)

	body := bytes.NewBufferString(`{"category":"unknown","title":"demo","content":"content"}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/messages", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	resp := decodeSiteMessageResponse(t, recorder)
	if !strings.Contains(resp.Message, "无效的消息分类") {
		t.Fatalf("unexpected response message: %s", resp.Message)
	}
}

func openSiteMessageHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := t.TempDir() + "/site-message-handler.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := createSiteMessageDictionarySchema(db); err != nil {
		t.Fatalf("create dictionary schema: %v", err)
	}
	return db
}

func newSiteMessageHandlerForTest(db *gorm.DB) *SiteMessageHandler {
	platformSettings := settingsrepo.NewPlatformSettingsRepositoryWithDB(db)
	return NewSiteMessageHandlerWithDeps(SiteMessageHandlerDeps{
		SiteMessageRepo: engagementrepo.NewSiteMessageRepositoryWithDeps(engagementrepo.SiteMessageRepositoryDeps{
			DB:               db,
			PlatformSettings: platformSettings,
		}),
		DictionaryRepo:       opsrepo.NewDictionaryRepositoryWithDB(db),
		PlatformSettingsRepo: platformSettings,
		EventBus:             platformevents.NewMessageEventBus(),
		TenantRepo:           accessrepo.NewTenantRepositoryWithDB(db),
		UserRepo:             accessrepo.NewUserRepositoryWithDB(db),
	})
}

func createSiteMessageDictionarySchema(db *gorm.DB) error {
	statements := []string{
		`CREATE TABLE sys_dictionaries (
			id TEXT PRIMARY KEY,
			dict_type TEXT NOT NULL,
			dict_key TEXT NOT NULL,
			label TEXT NOT NULL,
			label_en TEXT,
			color TEXT,
			tag_color TEXT,
			badge TEXT,
			icon TEXT,
			bg TEXT,
			extra TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_system INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_dict_type_key ON sys_dictionaries(dict_type, dict_key)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedSiteMessageDictionaryItem(t *testing.T, db *gorm.DB, dictType, dictKey, label string, sortOrder int, active bool) {
	t.Helper()
	item := opsmodel.Dictionary{
		ID:        uuid.New(),
		DictType:  dictType,
		DictKey:   dictKey,
		Label:     label,
		SortOrder: sortOrder,
		IsActive:  active,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create dictionary item: %v", err)
	}
}

func decodeSiteMessageResponse(t *testing.T, recorder *httptest.ResponseRecorder) respPkg.Response {
	t.Helper()
	var resp respPkg.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return resp
}

func decodeSiteMessageCategoryItems(t *testing.T, data any) []model.SiteMessageCategoryInfo {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal response data: %v", err)
	}
	var items []model.SiteMessageCategoryInfo
	if err := json.Unmarshal(payload, &items); err != nil {
		t.Fatalf("decode category items: %v; payload=%s", err, string(payload))
	}
	return items
}
