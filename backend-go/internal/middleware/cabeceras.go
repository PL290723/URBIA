package middleware

import (
	"slices"

	"github.com/gin-gonic/gin"
)

// CabecerasSeguridad agrega encabezados defensivos a todas las respuestas (HU 09.1).
//
// La API solo devuelve JSON e imágenes, así que la CSP más estricta no estorba.
// Las rutasHTML (el visor Swagger UI) quedan sin CSP porque cargan scripts del CDN.
// Cache-Control es el valor por defecto: los handlers que necesitan otro
// (documentación, imágenes) lo sobrescriben.
func CabecerasSeguridad(rutasHTML ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		if !slices.Contains(rutasHTML, c.Request.URL.Path) {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}
		c.Next()
	}
}
