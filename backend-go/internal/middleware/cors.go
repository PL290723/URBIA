package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/response"
)

const (
	corsMetodos     = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsEncabezados = "Authorization, Content-Type, X-Request-ID"
	corsExpuestos   = "X-Request-ID, Location, Content-Disposition, Retry-After"
	corsMaxAge      = "600" // segundos que el navegador recuerda el preflight
)

// CORS solo deja que los navegadores de los orígenes listados lean la API (HU 09.1).
//
//   - Sin encabezado Origin (Bruno, curl, app nativa) no hace nada: CORS es una
//     regla del navegador, no un control de acceso. El control real es el JWT.
//   - Origen permitido: agrega Access-Control-Allow-Origin con ESE origen (nunca "*").
//   - Preflight de un origen no permitido: 403, para que el rechazo sea visible.
//   - Petición normal de un origen no permitido: se atiende sin encabezados CORS
//     y el navegador bloquea la lectura. Como la API usa "Authorization: Bearer"
//     (no cookies), un sitio ajeno no puede mandar el token sin pasar el preflight.
//
// No se envía Access-Control-Allow-Credentials: la API no usa cookies.
func CORS(origenes []string) gin.HandlerFunc {
	permitidos := make(map[string]struct{}, len(origenes))
	for _, o := range origenes {
		permitidos[strings.ToLower(o)] = struct{}{}
	}

	return func(c *gin.Context) {
		origen := c.GetHeader("Origin")
		if origen == "" {
			c.Next()
			return
		}

		h := c.Writer.Header()
		h.Add("Vary", "Origin")
		_, permitido := permitidos[strings.ToLower(origen)]
		preflight := c.Request.Method == http.MethodOptions &&
			c.GetHeader("Access-Control-Request-Method") != ""

		if !permitido {
			if preflight {
				response.Error(c, http.StatusForbidden, response.CodeForbidden, "Origen no permitido")
				return
			}
			c.Next()
			return
		}

		h.Set("Access-Control-Allow-Origin", origen)
		h.Set("Access-Control-Expose-Headers", corsExpuestos)
		if preflight {
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
			h.Set("Access-Control-Allow-Methods", corsMetodos)
			h.Set("Access-Control-Allow-Headers", corsEncabezados)
			h.Set("Access-Control-Max-Age", corsMaxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
