package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type incidentErrorResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func TestRespondPluginIncidentErrorNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	respondPluginIncidentError(c, "获取工单详情失败", incidentrepo.ErrIncidentNotFound)

	assertIncidentErrorResponse(t, recorder, http.StatusNotFound, "工单不存在")
}

func TestRespondPluginIncidentErrorInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	respondPluginIncidentError(c, "关闭工单失败", errors.New("db unavailable"))

	assertIncidentErrorResponse(t, recorder, http.StatusInternalServerError, "关闭工单失败")
}

func TestRespondPluginIncidentErrorWritebackFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	logID := uuid.New()
	statusCode := http.StatusBadGateway

	respondPluginIncidentError(c, "关闭工单失败", &plugin.IncidentCloseWritebackError{
		Cause:              errors.New("source rejected close"),
		WritebackLogID:     &logID,
		ResponseStatusCode: &statusCode,
		Detail:             "String too long",
	})

	resp := assertIncidentErrorResponse(t, recorder, http.StatusBadGateway, "源系统回写失败（HTTP 502）：String too long。回写记录ID："+logID.String())
	if resp.Details["writeback_log_id"] != logID.String() {
		t.Fatalf("details.writeback_log_id = %#v, want %s", resp.Details["writeback_log_id"], logID.String())
	}
	if resp.Details["response_status_code"] != float64(http.StatusBadGateway) {
		t.Fatalf("details.response_status_code = %#v, want 502", resp.Details["response_status_code"])
	}
	if resp.Details["reason"] != "String too long" {
		t.Fatalf("details.reason = %#v, want source reason", resp.Details["reason"])
	}
}

func assertIncidentErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantMessage string) incidentErrorResponse {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d", recorder.Code, wantStatus)
	}

	var resp incidentErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Message != wantMessage {
		t.Fatalf("message = %q, want %q", resp.Message, wantMessage)
	}
	return resp
}
