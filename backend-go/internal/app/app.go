// Package app arma la API completa: repository → service → handler → router.
//
// Vive fuera de main para que las pruebas de integración (HU 08.2) levanten
// exactamente el mismo ensamblado que producción, cambiando solo las
// dependencias externas (PostgreSQL, almacenamiento y servicio de inferencia).
package app

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/imagenes"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/router"
	"github.com/PL290723/URBIA/backend-go/internal/services"
	"github.com/PL290723/URBIA/backend-go/internal/storage"
)

// Dependencias son las piezas externas que la API necesita ya creadas.
type Dependencias struct {
	Log        *slog.Logger
	Pool       *pgxpool.Pool
	Almacen    *storage.Local
	Tokens     *auth.TokenManager
	Inferencia *services.InferenciaMonitor // health de Python (INFERENCE_URL)

	// Detector es el cliente gRPC de Python. nil = no configurado: los reportes
	// se guardan con la sugerencia "no_disponible". OJO: asignar un
	// *clients.ClienteDeteccion nil haría la interfaz distinta de nil.
	Detector services.Detector

	Cercania  dominio.ParametrosCercania
	Prioridad dominio.ConfigPrioridad
	Version   string
	Inicio    time.Time
	Seguridad router.Seguridad // CORS y límite de login (HU 09.1); el valor cero es seguro
}

// NuevoRouter crea repositories, services y handlers y devuelve el router listo.
func NuevoRouter(d Dependencias) (*gin.Engine, error) {
	if d.Log == nil || d.Pool == nil || d.Almacen == nil || d.Tokens == nil || d.Inferencia == nil {
		return nil, errors.New("app: faltan dependencias obligatorias")
	}
	log := d.Log

	healthRepo := repositories.NewHealthRepository(d.Pool)
	usuarioRepo := repositories.NewUsuarioRepository(d.Pool)

	healthSvc := services.NewHealthService(healthRepo, d.Inferencia, log, d.Version, d.Inicio)
	usuarioSvc := services.NewUsuarioService(usuarioRepo)
	authSvc, err := services.NewAuthService(usuarioRepo, d.Tokens)
	if err != nil {
		return nil, err
	}

	archivoRepo := repositories.NewArchivoRepository(d.Pool)
	saneador := imagenes.NewSaneador(imagenes.LimitesPorDefecto(), 2) // máx. 2 imágenes procesándose a la vez
	archivoSvc := services.NewArchivoService(archivoRepo, d.Almacen, saneador, log)

	reporteRepo := repositories.NewReporteRepository(d.Pool)
	categoriaRepo := repositories.NewCategoriaRepository(d.Pool)
	deteccionRepo := repositories.NewDeteccionRepository(d.Pool)
	sugerenciaSvc := services.NewSugerenciaService(d.Detector, deteccionRepo, categoriaRepo, log)
	reporteSvc := services.NewReporteService(reporteRepo, categoriaRepo, archivoSvc, sugerenciaSvc, log)

	clasificacionRepo := repositories.NewClasificacionRepository(d.Pool)
	clasificacionSvc := services.NewClasificacionService(clasificacionRepo, deteccionRepo, categoriaRepo, log)
	consultaRepo := repositories.NewConsultaReporteRepository(d.Pool)
	consultaSvc := services.NewConsultaReporteService(consultaRepo, log)
	cercanosRepo := repositories.NewCercanosRepository(d.Pool)
	cercanosSvc := services.NewCercanosService(cercanosRepo, d.Cercania, log)
	prioridadRepo := repositories.NewPrioridadRepository(d.Pool)
	prioridadSvc := services.NewPrioridadService(prioridadRepo, d.Prioridad, d.Cercania, log)
	estadoRepo := repositories.NewEstadoRepository(d.Pool)
	estadoSvc := services.NewEstadoService(estadoRepo, log)
	estadisticasRepo := repositories.NewEstadisticasRepository(d.Pool)
	estadisticasSvc := services.NewEstadisticasService(estadisticasRepo, log)

	return router.New(log, d.Tokens, router.Handlers{
		Health:        handlers.NewHealthHandler(healthSvc),
		Auth:          handlers.NewAuthHandler(authSvc),
		Usuario:       handlers.NewUsuarioHandler(usuarioSvc),
		Archivo:       handlers.NewArchivoHandler(archivoSvc),
		Reporte:       handlers.NewReporteHandler(reporteSvc),
		Clasificacion: handlers.NewClasificacionHandler(clasificacionSvc),
		Consulta:      handlers.NewConsultaReporteHandler(consultaSvc),
		Cercanos:      handlers.NewCercanosHandler(cercanosSvc),
		Prioridad:     handlers.NewPrioridadHandler(prioridadSvc),
		Estado:        handlers.NewEstadoHandler(estadoSvc),
		Estadisticas:  handlers.NewEstadisticasHandler(estadisticasSvc),
	}, d.Seguridad), nil
}
