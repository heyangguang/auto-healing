package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	respPkg "github.com/company/auto-healing/internal/pkg/response"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	pluginSvc "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCMDBHandlerGetCMDBItemRejectsInvalidID(t *testing.T) {
	handler := &CMDBHandler{}
	router := gin.New()
	router.GET("/cmdb/:id", handler.GetCMDBItem)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cmdb/bad-id", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestCMDBHandlerListIDsAndStats(t *testing.T) {
	db := openCMDBHandlerTestDB(t)
	createCMDBHandlerSchema(t, db)

	tenantID := uuid.New()
	now := time.Now().UTC()
	mustExecCMDBHandlerSQL(t, db, `
		INSERT INTO cmdb_items (id, tenant_id, external_id, name, type, status, ip_address, hostname, environment, source_plugin_name, raw_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), "ext-1", "db-01", "server", "active", "10.0.0.1", "db-01", "production", "plugin-a", "{}", now, now)
	mustExecCMDBHandlerSQL(t, db, `
		INSERT INTO cmdb_items (id, tenant_id, external_id, name, type, status, ip_address, hostname, environment, source_plugin_name, raw_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), tenantID.String(), "ext-2", "db-02", "vm", "maintenance", "10.0.0.2", "db-02", "staging", "plugin-b", "{}", now, now)

	service := pluginSvc.NewCMDBServiceWithDeps(pluginSvc.CMDBServiceDeps{
		CMDBRepo: cmdbrepo.NewCMDBItemRepositoryWithDB(db),
	})
	handler := NewCMDBHandlerWithDeps(CMDBHandlerDeps{
		Service:       service,
		SecretService: &secretsSvc.Service{},
	})

	router := newCMDBHandlerTestRouter(tenantID)
	router.GET("/cmdb/ids", handler.ListCMDBItemIDs)
	router.GET("/cmdb/stats", handler.GetCMDBStats)

	idsResp := issueCMDBHandlerRequest(t, router, "/cmdb/ids?status=active")
	if idsResp.Code != respPkg.CodeSuccess {
		t.Fatalf("ids code = %d, want %d", idsResp.Code, respPkg.CodeSuccess)
	}
	if total := int64(*idsResp.Total); total != 1 {
		t.Fatalf("ids total = %d, want 1", total)
	}
	items := decodeCMDBCollectionItems(t, idsResp.Data)
	if len(items) != 1 || items[0]["status"] != "active" {
		t.Fatalf("unexpected ids payload: %#v", items)
	}

	statsResp := issueCMDBHandlerRequest(t, router, "/cmdb/stats")
	if statsResp.Code != respPkg.CodeSuccess {
		t.Fatalf("stats code = %d, want %d", statsResp.Code, respPkg.CodeSuccess)
	}
	stats := decodeCMDBStatsData(t, statsResp.Data)
	if got := int64(stats["total"].(float64)); got != 2 {
		t.Fatalf("stats.total = %d, want 2", got)
	}
}

func newCMDBHandlerTestRouter(tenantID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(platformrepo.WithTenantID(c.Request.Context(), tenantID))
		c.Next()
	})
	return router
}

func openCMDBHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func createCMDBHandlerSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExecCMDBHandlerSQL(t, db, `
		CREATE TABLE cmdb_items (
			id TEXT PRIMARY KEY NOT NULL,
			tenant_id TEXT,
			plugin_id TEXT,
			source_plugin_name TEXT,
			external_id TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT,
			status TEXT,
			ip_address TEXT,
			hostname TEXT,
			os TEXT,
			os_version TEXT,
			cpu TEXT,
			memory TEXT,
			disk TEXT,
			location TEXT,
			owner TEXT,
			environment TEXT,
			manufacturer TEXT,
			model TEXT,
			serial_number TEXT,
			department TEXT,
			dependencies TEXT,
			tags TEXT,
			raw_data TEXT NOT NULL,
			source_created_at DATETIME,
			source_updated_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			maintenance_reason TEXT,
			maintenance_start_at DATETIME,
			maintenance_end_at DATETIME
		)
	`)
}

func mustExecCMDBHandlerSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}

func issueCMDBHandlerRequest(t *testing.T, router http.Handler, path string) respPkg.Response {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(recorder, req)

	var resp respPkg.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return resp
}

func decodeCMDBCollectionItems(t *testing.T, data any) []map[string]any {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal cmdb ids data: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil {
		t.Fatalf("unmarshal cmdb ids data: %v; payload=%s", err, string(payload))
	}
	return items
}

func decodeCMDBStatsData(t *testing.T, data any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal cmdb stats data: %v", err)
	}
	var stats map[string]any
	if err := json.Unmarshal(payload, &stats); err != nil {
		t.Fatalf("unmarshal cmdb stats data: %v; payload=%s", err, string(payload))
	}
	return stats
}
