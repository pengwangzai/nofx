package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nofx/bootstrap"
	"github.com/nofx/config"
	"github.com/nofx/logger"
	"github.com/nofx/api"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Init(cfg.Logging)

	// Initialize bootstrap
	ctx, err := bootstrap.NewContext(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize bootstrap context: %v", err)
	}

	// Initialize API server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := api.NewServer(ctx, fmt.Sprintf("%s:%s", cfg.Server.Host, port))

	// Start server
	log.Printf("Server starting on %s:%s", cfg.Server.Host, port)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}