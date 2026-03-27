package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/ops/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	opsservice "github.com/company/auto-healing/internal/modules/ops/service"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestParseDictionaryTypesTrimsWhitespace(t *testing.T) {
	got := parseDictionaryTypes("instance_status, node_type , ,audit")
	want := []string{"instance_status", "node_type", "audit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDictionaryTypes() = %#v, want %#v", got, want)
	}
}

func TestListDictionariesRejectsInvalidActiveOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/dictionaries", (&DictionaryHandler{}).ListDictionaries)

	req := httptest.NewRequest(http.MethodGet, "/dictionaries?active_only=maybe", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestApplyDictionaryPatchPreservesOmittedFields(t *testing.T) {
	label := "new"
	existing := &model.Dictionary{
		Label:     "old",
		SortOrder: 9,
		IsActive:  true,
		Color:     "blue",
	}

	applyDictionaryPatch(existing, &updateDictionaryRequest{Label: &label})

	if existing.Label != "new" {
		t.Fatalf("label = %q, want new", existing.Label)
	}
	if existing.SortOrder != 9 {
		t.Fatalf("sort_order = %d, want 9", existing.SortOrder)
	}
	if !existing.IsActive {
		t.Fatal("is_active changed unexpectedly")
	}
	if existing.Color != "blue" {
		t.Fatalf("color = %q, want blue", existing.Color)
	}
}

func TestListDictionariesReturnsGroupedMap(t *testing.T) {
	router := newDictionaryHandlerTestRouter(t)

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/platform/dictionaries",
		strings.NewReader(`{"dict_type":"acc_test_type","dict_key":"case_a","label":"Case A","label_en":"Case A","sort_order":1,"is_active":true}`),
	)
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRecorder.Code, http.StatusCreated, createRecorder.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/common/dictionaries?types=acc_test_type&active_only=true", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload struct {
		Data map[string][]model.Dictionary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items := payload.Data["acc_test_type"]
	if len(items) != 1 {
		t.Fatalf("len(data[acc_test_type]) = %d, want 1; body=%s", len(items), recorder.Body.String())
	}
	if items[0].DictKey != "case_a" {
		t.Fatalf("dict_key = %q, want %q", items[0].DictKey, "case_a")
	}
}

func TestListTypesReturnsArray(t *testing.T) {
	db := openDictionaryHandlerTestDB(t)
	seedDictionaryItem(t, db, model.Dictionary{
		ID:        uuid.New(),
		DictType:  "alpha",
		DictKey:   "one",
		Label:     "One",
		IsActive:  true,
		SortOrder: 1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	router := newDictionaryHandlerTestRouterWithDB(t, db)
	req := httptest.NewRequest(http.MethodGet, "/common/dictionaries/types", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list types status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload struct {
		Data []opsrepo.DictTypeInfo `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1; body=%s", len(payload.Data), recorder.Body.String())
	}
	if payload.Data[0].DictType != "alpha" {
		t.Fatalf("data[0].dict_type = %q, want %q", payload.Data[0].DictType, "alpha")
	}
	if payload.Data[0].Count != 1 {
		t.Fatalf("data[0].count = %d, want 1", payload.Data[0].Count)
	}
}

func newDictionaryHandlerTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newDictionaryHandlerTestRouterWithDB(t, openDictionaryHandlerTestDB(t))
}

func newDictionaryHandlerTestRouterWithDB(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = origDB
	})
	logger.Init(&config.LogConfig{})
	gin.SetMode(gin.TestMode)

	h := NewDictionaryHandlerWithDeps(DictionaryHandlerDeps{
		Service: opsservice.NewDictionaryServiceWithDeps(opsservice.DictionaryServiceDeps{
			Repo: opsrepo.NewDictionaryRepositoryWithDB(db),
		}),
	})
	router := gin.New()
	router.GET("/common/dictionaries", h.ListDictionaries)
	router.GET("/common/dictionaries/types", h.ListTypes)
	router.POST("/platform/dictionaries", h.CreateDictionary)
	return router
}

func openDictionaryHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := t.TempDir() + "/dictionary-handler.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := createDictionarySchema(db); err != nil {
		t.Fatalf("create dictionary schema: %v", err)
	}
	return db
}

func seedDictionaryItem(t *testing.T, db *gorm.DB, item model.Dictionary) {
	t.Helper()
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create dictionary item: %v", err)
	}
}

func createDictionarySchema(db *gorm.DB) error {
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
