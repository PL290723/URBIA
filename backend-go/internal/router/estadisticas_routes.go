package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
)

// Las estadísticas del panel son solo para administradores (HU 07.4).
func registerEstadisticasRoutes(rg *gin.RouterGroup, h *handlers.EstadisticasHandler, requireAuth gin.HandlerFunc) {
	rg.GET("/estadisticas", requireAuth, middleware.RequirePermiso(dominio.Rol.PuedeVerEstadisticas), h.Calcular)
}
