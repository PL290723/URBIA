package middleware

import (
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// parametrosSensibles nunca se escriben en el log con su valor (HU 09.1):
// credenciales que un cliente mal hecho pudiera mandar en la URL y coordenadas
// exactas (la ubicación de un reporte es un dato personal del ciudadano).
var parametrosSensibles = map[string]bool{
	"token": true, "access_token": true, "jwt": true, "authorization": true,
	"contrasena": true, "password": true, "correo": true, "email": true,
	"lat": true, "latitud": true, "lon": true, "lng": true, "longitud": true, "bbox": true,
}

const valorOculto = "***"

func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if q := c.Request.URL.RawQuery; q != "" {
			path += "?" + consultaSegura(q)
		}

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"request_id", c.GetString(RequestIDKey),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case status >= 500:
			log.Error("solicitud", attrs...)
		case status >= 400:
			log.Warn("solicitud", attrs...)
		case strings.HasPrefix(c.Request.URL.Path, "/api/v1/health") || c.Request.URL.Path == "/api/health":
			log.Debug("solicitud", attrs...) // Docker lo consulta seguido; no saturar el log
		default:
			log.Info("solicitud", attrs...)
		}
	}
}

// consultaSegura reescribe el query string ocultando los valores sensibles.
// Ejemplo: "bbox=-100.9,20.5,-100.8,20.6&pagina=2" -> "bbox=***&pagina=2".
func consultaSegura(raw string) string {
	valores, err := url.ParseQuery(raw)
	if err != nil {
		return "(consulta ilegible omitida)"
	}
	claves := make([]string, 0, len(valores))
	for k := range valores {
		claves = append(claves, k)
	}
	slices.Sort(claves)

	partes := make([]string, 0, len(claves))
	for _, k := range claves {
		for _, v := range valores[k] {
			if parametrosSensibles[strings.ToLower(k)] {
				partes = append(partes, url.QueryEscape(k)+"="+valorOculto)
				continue
			}
			partes = append(partes, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(partes, "&")
}
