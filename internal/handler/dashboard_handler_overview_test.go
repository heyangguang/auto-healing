package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type dashboardTestContextKey string

func TestLoadDashboardSectionsFromLoadersPassesContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), dashboardTestContextKey("tenant"), "tenant-a")
	loaders := map[string]dashboardSectionFunc{
		"users": func(ctx context.Context) (interface{}, error) {
			return ctx.Value(dashboardTestContextKey("tenant")), nil
		},
		"git": func(ctx context.Context) (interface{}, error) {
			return ctx.Value(dashboardTestContextKey("tenant")), nil
		},
	}

	result, err := loadDashboardSectionsFromLoaders(ctx, loaders)
	if err != nil {
		t.Fatalf("loadDashboardSectionsFromLoaders() error = %v", err)
	}
	if result["users"] != "tenant-a" || result["git"] != "tenant-a" {
		t.Fatalf("loadDashboardSectionsFromLoaders() result = %#v", result)
	}
}

func TestLoadDashboardSectionsFromLoadersReturnsFirstError(t *testing.T) {
	expected := errors.New("boom")
	loaders := map[string]dashboardSectionFunc{
		"users": func(context.Context) (interface{}, error) { return nil, expected },
		"git":   func(context.Context) (interface{}, error) { return "ok", nil },
	}

	result, err := loadDashboardSectionsFromLoaders(context.Background(), loaders)
	if !errors.Is(err, expected) {
		t.Fatalf("loadDashboardSectionsFromLoaders() error = %v, want %v", err, expected)
	}
	if _, ok := result["git"]; !ok {
		t.Fatalf("loadDashboardSectionsFromLoaders() missing successful section result: %#v", result)
	}
}

func TestLoadDashboardSectionsFromLoadersJoinsErrorsAndRecoversPanics(t *testing.T) {
	expected := errors.New("boom")
	loaders := map[string]dashboardSectionFunc{
		"users": func(context.Context) (interface{}, error) { return nil, expected },
		"git":   func(context.Context) (interface{}, error) { panic("panic-boom") },
		"cmdb":  func(context.Context) (interface{}, error) { return "ok", nil },
	}

	result, err := loadDashboardSectionsFromLoaders(context.Background(), loaders)
	if !errors.Is(err, expected) {
		t.Fatalf("loadDashboardSectionsFromLoaders() error = %v, want joined error containing %v", err, expected)
	}
	if err == nil || !strings.Contains(err.Error(), "panic: panic-boom") {
		t.Fatalf("loadDashboardSectionsFromLoaders() error = %v, want panic details", err)
	}
	if result["cmdb"] != "ok" {
		t.Fatalf("loadDashboardSectionsFromLoaders() result = %#v, want successful section preserved", result)
	}
}
