package api

import (
	"github.com/gin-gonic/gin"
	"github.com/znsio/specmatic-order-bff-go/internal/handlers"
	"github.com/znsio/specmatic-order-bff-go/internal/middleware"
	"github.com/znsio/specmatic-order-bff-go/internal/services"
)

func SetupRouter(backendService *services.BackendService) *gin.Engine {
	r := gin.Default()

	productController := &handlers.ProductController{
		BackendService: backendService,
	}

	orderController := &handlers.OrderController{
		BackendService: backendService,
	}

	// Health check
	r.GET("/health", handlers.HealthCheck)

	// Product routes
	r.GET("/findAvailableProducts", middleware.RequirePageSize(), productController.FetchAvailableProducts)
	r.POST("/products", productController.CreateProduct)

	// Order routes
	r.POST("/orders", orderController.CreateOrder)

	return r
}
