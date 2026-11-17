package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
)

// Cambiar el estado exige token y rol administrador (HU 07.2).
func registerEstadoRoutes(rg *gin.RouterGroup, h *handlers.EstadoHandler, requireAuth gin.HandlerFunc) {
	rg.PATCH("/reportes/:id/estado", requireAuth,
		middleware.RequirePermiso(dominio.Rol.PuedeCambiarEstado), h.Cambiar)
}
