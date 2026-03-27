package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/response"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	platformrepo "github.com/company/auto-healing/internal/platform/repositoryx"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type incidentCloseE2EResponse struct {
	Message       string `json:"message"`
	LocalStatus   string `json:"local_status"`
	SourceUpdated bool   `json:"source_updated"`
}

func TestIncidentCloseEndToEnd(t *testing.T) {
	db := newPluginIncidentHandlerTestDB(t)
	createPluginIncidentSchema(t, db)
	bindPluginIncidentHandlerTestDB(t, db)

	closeRequest := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode close payload: %v", err)
		}
		payload["method"] = r.Method
		payload["path"] = r.URL.Path
		closeRequest <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tenantID := uuid.New()
	pluginID := uuid.New()
	incidentID := uuid.New()
	insertPluginIncidentPlugin(t, db, tenantID, pluginID, server.URL)
	insertPluginIncidentRecord(t, db, pluginIncidentRow{
		ID:         incidentID,
		TenantID:   tenantID,
		PluginID:   &pluginID,
		ExternalID: "INC-42",
		Title:      "database alert",
		Status:     "open",
		Healing:    "pending",
	})

	handler := newPluginIncidentHandlerTestHandler(t, db)
	router := newPluginIncidentHandlerRouter(tenantID, []string{incidentSyncPermission})
	router.POST("/incidents/:id/close", middleware.RequirePermission(incidentSyncPermission), handler.CloseIncident)

	recorder := issuePluginIncidentRequest(router, http.MethodPost, "/incidents/"+incidentID.String()+"/close", `{"resolution":"fixed","work_notes":"closed by e2e","close_code":"auto","close_status":"closed"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	payload := decodeIncidentCloseE2EResponse(t, recorder)
	if !payload.SourceUpdated {
		t.Fatal("source_updated = false, want true")
	}
	if payload.LocalStatus != "healed" {
		t.Fatalf("local_status = %q, want healed", payload.LocalStatus)
	}

	req := waitIncidentCloseRequest(t, closeRequest)
	if req["external_id"] != "INC-42" {
		t.Fatalf("external_id = %v, want INC-42", req["external_id"])
	}
	if req["path"] != "/close/INC-42" {
		t.Fatalf("path = %v, want /close/INC-42", req["path"])
	}

	ctx := platformrepo.WithTenantID(context.Background(), tenantID)
	incident, err := incidentrepo.NewIncidentRepositoryWithDB(db).GetByID(ctx, incidentID)
	if err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "closed" {
		t.Fatalf("status = %q, want closed", incident.Status)
	}
	if incident.HealingStatus != "healed" {
		t.Fatalf("healing_status = %q, want healed", incident.HealingStatus)
	}
}

func insertPluginIncidentPlugin(t *testing.T, db *gorm.DB, tenantID, pluginID uuid.UUID, baseURL string) {
	t.Helper()

	now := time.Now().UTC()
	configJSON := `{"close_incident_url":"` + baseURL + `/close/{external_id}","close_incident_method":"POST"}`
	mustExecPluginIncidentSQL(t, db, `
		INSERT INTO plugins (
			id, tenant_id, name, type, version, config, field_mapping, sync_enabled,
			sync_interval_minutes, max_failures, consecutive_failures, status, created_at, updated_at
		) VALUES (?, ?, 'itsm-plugin', 'itsm', '1.0.0', ?, '{}', 1, 5, 5, 0, 'active', ?, ?)
	`, pluginID.String(), tenantID.String(), configJSON, now, now)
}

func decodeIncidentCloseE2EResponse(t *testing.T, recorder *httptest.ResponseRecorder) incidentCloseE2EResponse {
	t.Helper()

	var envelope struct {
		Code int                      `json:"code"`
		Data incidentCloseE2EResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != response.CodeSuccess {
		t.Fatalf("code = %d, want %d", envelope.Code, response.CodeSuccess)
	}
	return envelope.Data
}

func waitIncidentCloseRequest(t *testing.T, closeRequest <-chan map[string]any) map[string]any {
	t.Helper()

	select {
	case req := <-closeRequest:
		return req
	case <-time.After(time.Second):
		t.Fatal("expected close-back request")
		return nil
	}
}
