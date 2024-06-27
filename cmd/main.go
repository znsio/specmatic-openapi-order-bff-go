package main

import (
	"log"

	"github.com/znsio/specmatic-order-bff-go/internal/api"
	"github.com/znsio/specmatic-order-bff-go/internal/config"
	"github.com/znsio/specmatic-order-bff-go/internal/services"
)

var authToken = "API-TOKEN-SPEC"

func main() {
	// Load configuration from config.yaml
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Access configuration
	cfg := config.GetConfig()

	backendService := services.NewBackendService(cfg.BackendAPI, authToken)

	// setup router and start server
	r := api.SetupRouter(backendService)
	r.Run(":" + cfg.ServerPort)
}
