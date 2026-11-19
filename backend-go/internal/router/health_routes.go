package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/handlers"
)

func registerHealthRoutes(rg *gin.RouterGroup, h *handlers.HealthHandler) {
	rg.GET("/health", h.Check)
	rg.GET("/health/live", h.Live)
}
