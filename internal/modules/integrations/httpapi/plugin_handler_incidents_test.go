package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

type incidentErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func TestRespondPluginIncidentErrorNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	respondPluginIncidentError(c, "获取工单详情失败", repository.ErrIncidentNotFound)

	assertIncidentErrorResponse(t, recorder, http.StatusNotFound, "工单不存在")
}

func TestRespondPluginIncidentErrorInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	respondPluginIncidentError(c, "关闭工单失败", errors.New("db unavailable"))

	assertIncidentErrorResponse(t, recorder, http.StatusInternalServerError, "关闭工单失败")
}

func assertIncidentErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantMessage string) {
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
}
