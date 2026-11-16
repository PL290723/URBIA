package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
)

// Las sugerencias de duplicados son internas: solo administradores.
func registerCercanosRoutes(rg *gin.RouterGroup, h *handlers.CercanosHandler, requireAuth gin.HandlerFunc) {
	rg.GET("/reportes/:id/cercanos", requireAuth,
		middleware.RequirePermiso(dominio.Rol.PuedeRevisarDuplicados), h.Buscar)
}
