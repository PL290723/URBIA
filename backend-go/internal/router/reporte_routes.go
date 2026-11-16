package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
)

// Crear un reporte exige token y rol ciudadano (dominio.Rol.PuedeCrearReporte).
func registerReporteRoutes(rg *gin.RouterGroup, h *handlers.ReporteHandler, requireAuth gin.HandlerFunc) {
	rg.POST("/reportes", requireAuth, middleware.RequirePermiso(dominio.Rol.PuedeCrearReporte), h.Crear)
}
