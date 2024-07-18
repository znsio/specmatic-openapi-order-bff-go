package main

import (
	"fmt"
	"log"
	"net/url"

	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/api"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/config"
	"github.com/znsio/specmatic-order-bff-go/internal/com/store/order/bff/services"
)

var authToken = "API-TOKEN-SPEC"

func main() {
	// Load configuration from config.yaml
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	StartServer(cfg)
}

func StartServer(cfg *config.Config) {
	backendURL := url.URL{
		Scheme: "http",
		Host:   cfg.BackendHost + ":" + cfg.BackendPort,
	}
	fmt.Println("port received is ================ : ", cfg.BackendHost+":"+cfg.BackendPort)
	backendService := services.NewBackendService(backendURL.String(), authToken)

	// setup router and start server
	r := api.SetupRouter(backendService)
	r.Run(":" + cfg.BFFServerPort)
}
