package plugin

import (
	"testing"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

func TestMapToIncidentsMapsSourceTimes(t *testing.T) {
	svc := &Service{}
	records := svc.mapToIncidents([]map[string]interface{}{{
		"id":                "INC-1",
		"title":             "cpu high",
		"source_created_at": "2026-04-13T12:30:00Z",
		"source_updated_at": "2026-04-13 20:31:00",
	}}, model.JSON{})

	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].SourceCreatedAt.IsZero() {
		t.Fatal("expected incident source_created_at to be parsed")
	}
	wantUpdated := time.Date(2026, 4, 13, 20, 31, 0, 0, time.UTC)
	if !records[0].SourceUpdatedAt.Equal(wantUpdated) {
		t.Fatalf("SourceUpdatedAt = %v, want %v", records[0].SourceUpdatedAt, wantUpdated)
	}
}

func TestMapToCMDBItemsMapsSourceTimes(t *testing.T) {
	svc := &Service{}
	records := svc.mapToCMDBItems([]map[string]interface{}{{
		"id":                "Server::1",
		"name":              "app-01",
		"source_created_at": "2026-04-01",
		"source_updated_at": "2026-04-13T22:15:00Z",
	}}, model.JSON{})

	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].SourceCreatedAt.IsZero() {
		t.Fatal("expected cmdb source_created_at to be parsed")
	}
	if records[0].SourceUpdatedAt.IsZero() {
		t.Fatal("expected cmdb source_updated_at to be parsed")
	}
}
