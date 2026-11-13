package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/handlers"
)

// Las imágenes nunca se sirven como estáticos: toda ruta exige token y la
// política de acceso (autor del reporte o administrador) se aplica en el service.
func registerArchivoRoutes(rg *gin.RouterGroup, h *handlers.ArchivoHandler, requireAuth gin.HandlerFunc) {
	rg.POST("/reportes/:id/archivos", requireAuth, h.Subir)

	g := rg.Group("/archivos", requireAuth)
	g.GET("/:id", h.Descargar)
	g.DELETE("/:id", h.Eliminar)
}
