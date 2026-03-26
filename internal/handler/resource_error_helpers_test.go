package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
)

func TestRespondResourceErrorReturnsNotFoundForSentinel(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	respondResourceError(ctx, "GIT", "获取仓库失败", "仓库不存在", repository.ErrGitRepositoryNotFound, resourceErrorModeInternal, repository.ErrGitRepositoryNotFound)

	assertResponseCode(t, recorder, http.StatusNotFound, response.CodeNotFound)
}

func TestRespondResourceErrorReturnsBadRequestWhenConfigured(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	respondResourceError(ctx, "PLAYBOOK", "扫描失败", "Playbook不存在", repository.ErrPlaybookNotFound, resourceErrorModeBadRequest, errors.New("路径非法"))

	assertResponseCode(t, recorder, http.StatusBadRequest, response.CodeBadRequest)
}

func TestRespondResourceErrorReturnsInternalForUnexpectedError(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	respondResourceError(ctx, "SCHEDULE", "获取调度失败", "调度不存在", repository.ErrScheduleNotFound, resourceErrorModeInternal, errors.New("db down"))

	assertResponseCode(t, recorder, http.StatusInternalServerError, response.CodeInternal)
}

func assertResponseCode(t *testing.T, recorder *httptest.ResponseRecorder, wantHTTP, wantCode int) {
	t.Helper()

	if recorder.Code != wantHTTP {
		t.Fatalf("status = %d, want %d", recorder.Code, wantHTTP)
	}

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != wantCode {
		t.Fatalf("code = %d, want %d", resp.Code, wantCode)
	}
}
