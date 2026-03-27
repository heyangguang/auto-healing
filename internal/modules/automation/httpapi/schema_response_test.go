package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type schemaListResponse struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    []map[string]interface{} `json:"data"`
}

func TestExecutionSearchSchemasReturnTopLevelDataArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ExecutionHandler{}

	cases := []struct {
		name   string
		target func(*gin.Context)
	}{
		{name: "task search schema", target: handler.GetTaskSearchSchema},
		{name: "run search schema", target: handler.GetRunSearchSchema},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := issueSchemaRequest(t, tc.target)
			if len(resp.Data) == 0 {
				t.Fatal("data = empty, want schema entries")
			}
			if _, exists := resp.Data[0]["fields"]; exists {
				t.Fatalf("data[0] unexpectedly contains nested fields wrapper: %+v", resp.Data[0])
			}
		})
	}
}

func TestHealingSearchSchemasReturnTopLevelDataArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &HealingHandler{}

	cases := []struct {
		name   string
		target func(*gin.Context)
	}{
		{name: "flow search schema", target: handler.GetFlowSearchSchema},
		{name: "rule search schema", target: handler.GetRuleSearchSchema},
		{name: "instance search schema", target: handler.GetInstanceSearchSchema},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := issueSchemaRequest(t, tc.target)
			if len(resp.Data) == 0 {
				t.Fatal("data = empty, want schema entries")
			}
			if _, exists := resp.Data[0]["fields"]; exists {
				t.Fatalf("data[0] unexpectedly contains nested fields wrapper: %+v", resp.Data[0])
			}
		})
	}
}

func issueSchemaRequest(t *testing.T, target func(*gin.Context)) schemaListResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/schema", nil)

	target(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var resp schemaListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != 0 || resp.Message != "success" {
		t.Fatalf("response = %+v, want code=0 message=success", resp)
	}
	return resp
}
