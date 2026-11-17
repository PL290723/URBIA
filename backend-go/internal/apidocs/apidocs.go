// Package apidocs publica la especificación OpenAPI de la API (HU 08.1).
//
// openapi.yaml se escribe a mano y se compila dentro del binario con go:embed:
// no depende de archivos sueltos al desplegar. La prueba router/openapi_test.go
// falla si una ruta de Gin no está documentada (o al revés), así la
// documentación no se queda atrás de la implementación.
package apidocs

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	RutaEspecificacion = "/api/v1/openapi.yaml"
	RutaVisor          = "/api/v1/docs"
)

//go:embed openapi.yaml
var especificacion []byte

// Especificacion devuelve una copia del YAML (para pruebas y herramientas).
func Especificacion() []byte {
	return append([]byte(nil), especificacion...)
}

// ServirEspecificacion: GET /api/v1/openapi.yaml
func ServirEspecificacion(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", especificacion)
}

// ServirVisor: GET /api/v1/docs — Swagger UI desde el CDN de jsDelivr.
func ServirVisor(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(paginaVisor))
}

const paginaVisor = `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>URBIA API · Documentación</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>body { margin: 0; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <noscript>Activa JavaScript o descarga la especificación en <a href="` + RutaEspecificacion + `">` + RutaEspecificacion + `</a>.</noscript>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin="anonymous"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "` + RutaEspecificacion + `",
      dom_id: "#swagger-ui",
      deepLinking: true,
      persistAuthorization: true,
      displayRequestDuration: true,
      tryItOutEnabled: false
    });
  </script>
</body>
</html>`
