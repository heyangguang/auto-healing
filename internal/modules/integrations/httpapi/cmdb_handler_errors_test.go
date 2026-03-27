package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	pluginservice "github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

func TestRespondCMDBItemErrorNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	respondCMDBItemError(ctx, "获取 CMDB 详情失败", repository.ErrCMDBItemNotFound)

	assertCMDBErrorResponse(t, recorder, http.StatusNotFound, response.CodeNotFound, cmdbNotFoundMessage)
}

func TestRespondCMDBMaintenanceErrorOfflineReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	respondCMDBMaintenanceError(ctx, "进入维护模式失败", pluginservice.ErrCMDBOfflineMaintenanceForbidden)

	assertCMDBErrorResponse(t, recorder, http.StatusBadRequest, response.CodeBadRequest, pluginservice.ErrCMDBOfflineMaintenanceForbidden.Error())
}

func TestRespondCMDBMaintenanceErrorInternalReturnsServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	respondCMDBMaintenanceError(ctx, "退出维护模式失败", errors.New("db unavailable"))

	assertCMDBErrorResponse(t, recorder, http.StatusInternalServerError, response.CodeInternal, "退出维护模式失败")
}

func TestCMDBLookupFailureMessagePreservesInternalErrors(t *testing.T) {
	message := cmdbLookupFailureMessage(errors.New("db unavailable"))
	if message != "查询配置项失败: db unavailable" {
		t.Fatalf("message = %q, want %q", message, "查询配置项失败: db unavailable")
	}
}

func assertCMDBErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantHTTP, wantCode int, wantMessage string) {
	t.Helper()

	if recorder.Code != wantHTTP {
		t.Fatalf("status = %d, want %d", recorder.Code, wantHTTP)
	}

	var payload response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != wantCode {
		t.Fatalf("response code = %d, want %d", payload.Code, wantCode)
	}
	if payload.Message != wantMessage {
		t.Fatalf("response message = %q, want %q", payload.Message, wantMessage)
	}
}
