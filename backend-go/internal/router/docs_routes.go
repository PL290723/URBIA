package router

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/apidocs"
)

// La documentación OpenAPI es pública (HU 08.1): no expone datos, solo el contrato.
func registerDocsRoutes(r *gin.Engine) {
	r.GET(apidocs.RutaEspecificacion, apidocs.ServirEspecificacion)
	r.GET(apidocs.RutaVisor, apidocs.ServirVisor)
}
