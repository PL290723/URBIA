package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
)

// Confirmar la categoría exige token y rol ciudadano; que sea SU reporte lo revisa el service.
func registerClasificacionRoutes(rg *gin.RouterGroup, h *handlers.ClasificacionHandler, requireAuth gin.HandlerFunc) {
	rg.PUT("/reportes/:id/categoria", requireAuth,
		middleware.RequirePermiso(dominio.Rol.PuedeConfirmarCategoria), h.Confirmar)
}
