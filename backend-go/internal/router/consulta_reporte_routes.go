package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/handlers"
)

// Cualquier usuario autenticado consulta reportes; el service recorta los datos
// según su rol y si es o no el autor (dominio.Vista).
func registerConsultaReporteRoutes(rg *gin.RouterGroup, h *handlers.ConsultaReporteHandler, requireAuth gin.HandlerFunc) {
	rg.GET("/reportes", requireAuth, h.Listar)
	rg.GET("/reportes/:id", requireAuth, h.Detalle)
}
