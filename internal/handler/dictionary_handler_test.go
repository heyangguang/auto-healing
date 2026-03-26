package handler

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/company/auto-healing/internal/model"
	"github.com/gin-gonic/gin"
)

func TestParseDictionaryTypesTrimsWhitespace(t *testing.T) {
	got := parseDictionaryTypes("instance_status, node_type , ,audit")
	want := []string{"instance_status", "node_type", "audit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDictionaryTypes() = %#v, want %#v", got, want)
	}
}

func TestListDictionariesRejectsInvalidActiveOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/dictionaries", (&DictionaryHandler{}).ListDictionaries)

	req := httptest.NewRequest(http.MethodGet, "/dictionaries?active_only=maybe", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestApplyDictionaryPatchPreservesOmittedFields(t *testing.T) {
	label := "new"
	existing := &model.Dictionary{
		Label:     "old",
		SortOrder: 9,
		IsActive:  true,
		Color:     "blue",
	}

	applyDictionaryPatch(existing, &updateDictionaryRequest{Label: &label})

	if existing.Label != "new" {
		t.Fatalf("label = %q, want new", existing.Label)
	}
	if existing.SortOrder != 9 {
		t.Fatalf("sort_order = %d, want 9", existing.SortOrder)
	}
	if !existing.IsActive {
		t.Fatal("is_active changed unexpectedly")
	}
	if existing.Color != "blue" {
		t.Fatalf("color = %q, want blue", existing.Color)
	}
}
