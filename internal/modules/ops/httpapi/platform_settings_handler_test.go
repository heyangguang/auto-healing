package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateSettingValueRejectsInvalidJSON(t *testing.T) {
	err := validateSettingValue(model.SettingTypeJSON, "{")
	if err == nil {
		t.Fatal("validateSettingValue() error = nil, want invalid JSON error")
	}
}

func TestValidateSettingValueAcceptsValidJSON(t *testing.T) {
	err := validateSettingValue(model.SettingTypeJSON, `{"enabled":true}`)
	if err != nil {
		t.Fatalf("validateSettingValue() error = %v, want nil", err)
	}
}

func TestPlatformSettingsUpdateReturnsInternalOnLookupError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPlatformSettingsHandlerTestDB(t)
	origDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = origDB })

	router := gin.New()
	router.PUT("/settings/:key", NewPlatformSettingsHandler().UpdateSetting)

	req := httptest.NewRequest(http.MethodPut, "/settings/email.password", bytes.NewBufferString(`{"value":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func openPlatformSettingsHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:platform-settings-handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
