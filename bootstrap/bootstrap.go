package bootstrap

import (
	"github.com/nofx/config"
	"github.com/nofx/logger"
)

// Bootstrap handles application startup and initialization
func Bootstrap() (*Context, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Initialize logger
	logger.Init(cfg.Logging)

	// Create context
	ctx, err := NewContext(cfg)
	if err != nil {
		logger.Error("Failed to initialize application context: %v", err)
		return nil, err
	}

	logger.Info("Application bootstrapped successfully")
	return ctx, nil
}