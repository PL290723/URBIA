package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

type Handlers struct {
	Health        *handlers.HealthHandler
	Auth          *handlers.AuthHandler
	Usuario       *handlers.UsuarioHandler
	Archivo       *handlers.ArchivoHandler
	Reporte       *handlers.ReporteHandler
	Clasificacion *handlers.ClasificacionHandler
	Consulta      *handlers.ConsultaReporteHandler
	Cercanos      *handlers.CercanosHandler
	Prioridad     *handlers.PrioridadHandler
	Estado        *handlers.EstadoHandler
	Estadisticas  *handlers.EstadisticasHandler
}

func New(log *slog.Logger, tokens *auth.TokenManager, h Handlers) *gin.Engine {
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

	requireAuth := middleware.RequireAuth(tokens)
	// Alias de compatibilidad (HU 01.2 / 03.3 hablan de /api/health)
	r.GET("/api/health", h.Health.Check)

	v1 := r.Group("/api/v1")
	registerHealthRoutes(v1, h.Health)
	registerAuthRoutes(v1, h.Auth, requireAuth)
	registerAdminRoutes(v1, h.Usuario, requireAuth)
	registerArchivoRoutes(v1, h.Archivo, requireAuth)
	registerReporteRoutes(v1, h.Reporte, requireAuth)
	registerClasificacionRoutes(v1, h.Clasificacion, requireAuth)
	registerConsultaReporteRoutes(v1, h.Consulta, requireAuth)
	registerCercanosRoutes(v1, h.Cercanos, requireAuth)
	registerPrioridadRoutes(v1, h.Prioridad, requireAuth)
	registerEstadoRoutes(v1, h.Estado, requireAuth)
	registerEstadisticasRoutes(v1, h.Estadisticas, requireAuth)
	registerDocsRoutes(r)
	return r
}
