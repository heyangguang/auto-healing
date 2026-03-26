package provider

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/pkg/logger"
)

func init() {
	logger.Init(&config.LogConfig{
		Level: "error",
	})
}
