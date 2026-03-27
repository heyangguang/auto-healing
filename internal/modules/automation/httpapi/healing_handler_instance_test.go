package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/company/auto-healing/internal/modules/automation/model"
	healing "github.com/company/auto-healing/internal/modules/automation/service/healing"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (r *flushRecorder) Flush() {}

func TestBuildFlowInstanceListOptionsParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/instances?status=running&flow_id=11111111-1111-1111-1111-111111111111&rule_id=22222222-2222-2222-2222-222222222222&incident_id=33333333-3333-3333-3333-333333333333&current_node_id=node-1&approval_status=approved&created_from=2026-03-01&created_to=2026-03-02T03:04:05Z&min_nodes=2&max_nodes=8&min_failed_nodes=1&max_failed_nodes=3&has_error=1&sort_by=started_at&sort_order=asc", nil)

	opts := buildFlowInstanceListOptions(ctx, 2, 50)

	if opts.Page != 2 || opts.PageSize != 50 {
		t.Fatalf("pagination = (%d,%d), want (2,50)", opts.Page, opts.PageSize)
	}
	if opts.Status != "running" || opts.CurrentNodeID != "node-1" || opts.ApprovalStatus != "approved" {
		t.Fatalf("unexpected scalar fields: %+v", opts)
	}
	if opts.FlowID == nil || opts.RuleID == nil || opts.IncidentID == nil {
		t.Fatalf("uuid filters not parsed: %+v", opts)
	}
	if opts.CreatedFrom == nil || opts.CreatedTo == nil {
		t.Fatalf("time filters not parsed: %+v", opts)
	}
	if opts.MinNodes == nil || *opts.MinNodes != 2 || opts.MaxNodes == nil || *opts.MaxNodes != 8 {
		t.Fatalf("node filters not parsed: %+v", opts)
	}
	if opts.MinFailedNodes == nil || *opts.MinFailedNodes != 1 || opts.MaxFailedNodes == nil || *opts.MaxFailedNodes != 3 {
		t.Fatalf("failed node filters not parsed: %+v", opts)
	}
	if opts.HasError == nil || !*opts.HasError {
		t.Fatalf("has_error not parsed: %+v", opts)
	}
}

func TestBuildFlowInstanceListOptionsIgnoresInvalidOptionalValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/instances?flow_id=bad-uuid&created_from=nope&min_nodes=oops&has_error=false", nil)

	opts := buildFlowInstanceListOptions(ctx, 1, 20)

	if opts.FlowID != nil || opts.CreatedFrom != nil || opts.MinNodes != nil {
		t.Fatalf("invalid values should be ignored: %+v", opts)
	}
	if opts.HasError == nil || *opts.HasError {
		t.Fatalf("has_error=false should parse to false: %+v", opts)
	}
}

func TestWriteInitialInstanceEventsForRunningInstance(t *testing.T) {
	writer, recorder := newTestSSEWriter(t)
	instance := &model.FlowInstance{
		ID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Status: model.FlowInstanceStatusRunning,
	}

	terminal := writeInitialInstanceEvents(writer, instance)

	if terminal {
		t.Fatal("running instance should not be terminal")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: connected") {
		t.Fatalf("missing connected event: %s", body)
	}
	if strings.Contains(body, "event: flow_complete") {
		t.Fatalf("unexpected flow_complete event: %s", body)
	}
}

func TestWriteInitialInstanceEventsForTerminalInstance(t *testing.T) {
	writer, recorder := newTestSSEWriter(t)
	instance := &model.FlowInstance{
		ID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Status:       model.FlowInstanceStatusFailed,
		ErrorMessage: "boom",
	}

	terminal := writeInitialInstanceEvents(writer, instance)

	if !terminal {
		t.Fatal("failed instance should be terminal")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: connected") || !strings.Contains(body, "event: flow_complete") {
		t.Fatalf("missing expected events: %s", body)
	}
	if !strings.Contains(body, "\"message\":\"boom\"") {
		t.Fatalf("missing terminal error message: %s", body)
	}
}

func TestStreamInstanceEventsStopsOnFlowComplete(t *testing.T) {
	writer, recorder := newTestSSEWriter(t)
	eventCh := make(chan healing.Event, 2)
	eventCh <- healing.Event{Type: healing.EventNodeLog, Data: map[string]interface{}{"message": "step"}}
	eventCh <- healing.Event{Type: healing.EventFlowComplete, Data: map[string]interface{}{"status": "completed"}}

	streamInstanceEvents(context.Background(), writer, eventCh)

	body := recorder.Body.String()
	if !strings.Contains(body, "event: node_log") {
		t.Fatalf("missing node log event: %s", body)
	}
	if !strings.Contains(body, "event: flow_complete") {
		t.Fatalf("missing flow complete event: %s", body)
	}
}

func newTestSSEWriter(t *testing.T) (*SSEWriter, *flushRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/events", nil)
	writer, err := NewSSEWriter(ctx)
	if err != nil {
		t.Fatalf("NewSSEWriter() error = %v", err)
	}
	return writer, recorder
}
