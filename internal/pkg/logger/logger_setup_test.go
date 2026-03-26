package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/company/auto-healing/internal/config"
	"go.uber.org/zap"
)

func TestInitUsesConfiguredPathForCategoryLogs(t *testing.T) {
	restoreLoggerGlobals(t)

	tmpDir := t.TempDir()
	Init(&config.LogConfig{
		Level: "info",
		Console: config.ConsoleLogConfig{
			Enabled: false,
		},
		File: config.FileLogConfig{
			Enabled:    true,
			Path:       tmpDir,
			Filename:   "app.log",
			Format:     "json",
			MaxSize:    10,
			MaxBackups: 1,
			MaxAge:     1,
		},
	})

	writer := getCategoryWriter(CatSched)
	if writer == nil {
		t.Fatal("category writer should be initialized")
	}
	want := filepath.Join(tmpDir, categoryToFile[CatSched])
	if writer.Filename != want {
		t.Fatalf("writer path = %q, want %q", writer.Filename, want)
	}
}

func TestInitDisablesCategoryLogsWhenFileDisabled(t *testing.T) {
	restoreLoggerGlobals(t)

	Init(&config.LogConfig{
		Level: "info",
		Console: config.ConsoleLogConfig{
			Enabled: true,
			Format:  "text",
		},
		File: config.FileLogConfig{
			Enabled: false,
		},
	})

	if writer := getCategoryWriter(CatExec); writer != nil {
		t.Fatalf("category writer = %#v, want nil when file logging disabled", writer)
	}
}

func TestInitDisablesCategoryLogsWhenLogDirCreateFails(t *testing.T) {
	restoreLoggerGlobals(t)

	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	Init(&config.LogConfig{
		Level: "info",
		Console: config.ConsoleLogConfig{
			Enabled: true,
			Format:  "text",
		},
		File: config.FileLogConfig{
			Enabled:    true,
			Path:       filepath.Join(blocker, "logs"),
			Filename:   "app.log",
			Format:     "json",
			MaxSize:    10,
			MaxBackups: 1,
			MaxAge:     1,
		},
	})

	if writer := getCategoryWriter(CatSync); writer != nil {
		t.Fatalf("category writer = %#v, want nil when log dir create fails", writer)
	}
}

func restoreLoggerGlobals(t *testing.T) {
	t.Helper()

	oldLogger := logger
	oldSugar := sugar
	oldLogDir := logDir
	categoryMu.Lock()
	oldWriters := categoryWriters
	categoryMu.Unlock()

	t.Cleanup(func() {
		closeCategoryWriters()
		logger = oldLogger
		if logger == nil {
			logger = zap.NewNop()
		}
		sugar = oldSugar
		if sugar == nil {
			sugar = logger.Sugar()
		}
		logDir = oldLogDir

		categoryMu.Lock()
		categoryWriters = oldWriters
		categoryMu.Unlock()
	})
}
