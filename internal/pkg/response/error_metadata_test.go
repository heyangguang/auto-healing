package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type testMetadataError struct{}

func (testMetadataError) Error() string     { return "wrapped failure" }
func (testMetadataError) ErrorCode() string { return "TEST_CODE" }
func (testMetadataError) ErrorDetails() any { return map[string]string{"hint": "value"} }

func TestBadRequestFromErrIncludesMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	BadRequestFromErr(ctx, errors.New("outer: "+testMetadataError{}.Error()))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestErrorFromErrExtractsWrappedMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	err := errors.Join(errors.New("ignored"), testMetadataError{})
	ErrorFromErr(ctx, http.StatusBadRequest, CodeBadRequest, err)

	var resp Response
	if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &resp); decodeErr != nil {
		t.Fatalf("decode response: %v", decodeErr)
	}
	if resp.ErrorCode != "TEST_CODE" {
		t.Fatalf("unexpected error_code: %q", resp.ErrorCode)
	}
	details, ok := resp.Details.(map[string]any)
	if ok {
		if details["hint"] != "value" {
			t.Fatalf("unexpected details: %#v", details)
		}
	}
}
