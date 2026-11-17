package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/apidocs"
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

// Seguridad agrupa la configuración de los middlewares de seguridad (HU 09.1).
// El valor cero es seguro: sin CORS y con el límite de login por defecto.
type Seguridad struct {
	OrigenesCORS     []string
	LoginMaxIntentos int           // 0 = loginMaxIntentosDefecto
	LoginVentana     time.Duration // 0 = loginVentanaDefecto
}

const (
	loginMaxIntentosDefecto = 10
	loginVentanaDefecto     = 5 * time.Minute
)

func New(log *slog.Logger, tokens *auth.TokenManager, h Handlers, seg Seguridad) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	_ = r.SetTrustedProxies(nil) // c.ClientIP() = IP real de la conexión; no se confía en X-Forwarded-For

	// El orden importa: CORS va antes que NoRoute/NoMethod para contestar el
	// preflight OPTIONS, y LimiteCuerpo antes que cualquier handler que lea el cuerpo.
	r.Use(
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.Recovery(log),
		middleware.CabecerasSeguridad(apidocs.RutaVisor),
		middleware.CORS(seg.OrigenesCORS),
		middleware.LimiteCuerpo(),
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
	registerAuthRoutes(v1, h.Auth, requireAuth, middleware.LimiteIntentos(seg.loginMaxIntentos(), seg.loginVentana()))
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

func (s Seguridad) loginMaxIntentos() int {
	if s.LoginMaxIntentos > 0 {
		return s.LoginMaxIntentos
	}
	return loginMaxIntentosDefecto
}

func (s Seguridad) loginVentana() time.Duration {
	if s.LoginVentana > 0 {
		return s.LoginVentana
	}
	return loginVentanaDefecto
}
