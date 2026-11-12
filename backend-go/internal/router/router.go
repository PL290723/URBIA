package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

type Handlers struct {
	Health *handlers.HealthHandler
}

func New(log *slog.Logger, h Handlers) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	_ = r.SetTrustedProxies(nil)

	r.Use(
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.Recovery(log),
	)

	r.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Ruta no encontrada")
	})
	r.NoMethod(func(c *gin.Context) {
		response.Error(c, http.StatusMethodNotAllowed, response.CodeMethodNotAllowed, "Método no permitido")
	})

	v1 := r.Group("/api/v1")
	registerHealthRoutes(v1, h.Health)
	// Próximos módulos: registerAuthRoutes(v1, h.Auth), registerReporteRoutes(v1, h.Reporte)…

	return r
}
