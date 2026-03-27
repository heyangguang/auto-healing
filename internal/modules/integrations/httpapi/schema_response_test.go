package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type integrationSchemaListResponse struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    []map[string]interface{} `json:"data"`
}

func TestSearchSchemaHandlersReturnTopLevelDataArray(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		target func(*gin.Context)
	}{
		{name: "git repo schema", target: (&GitRepoHandler{}).GetSearchSchema},
		{name: "plugin schema", target: (&PluginHandler{}).GetPluginSearchSchema},
		{name: "incident schema", target: (&PluginHandler{}).GetIncidentSearchSchema},
		{name: "cmdb schema", target: (&CMDBHandler{}).GetCMDBSearchSchema},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/schema", nil)

			tc.target(ctx)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}

			var resp integrationSchemaListResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Code != 0 || resp.Message != "success" {
				t.Fatalf("response = %+v, want code=0 message=success", resp)
			}
			if len(resp.Data) == 0 {
				t.Fatal("data = empty, want schema entries")
			}
			if _, exists := resp.Data[0]["fields"]; exists {
				t.Fatalf("data[0] unexpectedly contains nested fields wrapper: %+v", resp.Data[0])
			}
		})
	}
}
