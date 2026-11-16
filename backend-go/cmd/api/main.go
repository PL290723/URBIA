package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/imagenes"
	"github.com/PL290723/URBIA/backend-go/internal/storage"
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/config"
	"github.com/PL290723/URBIA/backend-go/internal/db"
	"github.com/PL290723/URBIA/backend-go/internal/handlers"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/router"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

// version se sobreescribe al compilar: go build -ldflags "-X main.version=0.3.0" ./cmd/api
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("el servidor terminó con error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	inicio := time.Now()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuración inválida:\n%w", err)
	}

	log := newLogger(cfg)
	slog.SetDefault(log)
	gin.SetMode(cfg.GinMode)
	log.Info("iniciando URBIA API", append([]any{"version", version}, cfg.ResumenSeguro()...)...)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Info("conectado a PostgreSQL")

	almacen, err := storage.NewLocal(cfg.StorageDir, dominio.TamanoMaximoArchivo)
	if err != nil {
		return err
	}
	defer almacen.Close()
	log.Info("almacenamiento de imágenes listo", "dir", almacen.Dir())

	// Servicio de inferencia: se vigila en segundo plano y no bloquea el arranque.
	inferencia := services.NewInferenciaMonitor(
		clients.NewInferenciaClient(cfg.InferenceURL, cfg.InferenceTimeout),
		cfg.InferenceInterval, log,
	)
	inferencia.Iniciar(ctx)

	// Inyección de dependencias: repository → service → handler
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)

	healthRepo := repositories.NewHealthRepository(pool)
	usuarioRepo := repositories.NewUsuarioRepository(pool)

	healthSvc := services.NewHealthService(healthRepo, inferencia, log, version, inicio)
	usuarioSvc := services.NewUsuarioService(usuarioRepo)
	authSvc, err := services.NewAuthService(usuarioRepo, tokens)
	if err != nil {
		return err
	}
	archivoRepo := repositories.NewArchivoRepository(pool)
	saneador := imagenes.NewSaneador(imagenes.LimitesPorDefecto(), 2) // máx. 2 imágenes procesándose a la vez
	archivoSvc := services.NewArchivoService(archivoRepo, almacen, saneador, log)

	// Cliente gRPC de inferencia (HU 06.2). Sin INFERENCE_GRPC_ADDR la API funciona
	// igual y los reportes se guardan con la sugerencia "no_disponible".
	// Ojo: detector se queda en nil (interfaz vacía) si no está configurado.
	var detector services.Detector
	if cfg.InferenceGRPCAddr != "" {
		clienteDeteccion, err := clients.NuevoClienteDeteccion(cfg.InferenceGRPCAddr, cfg.InferenceGRPCTimeout)
		if err != nil {
			return err
		}
		defer clienteDeteccion.Cerrar()
		detector = clienteDeteccion
		log.Info("sugerencias de categoría con IA activas", "grpc", cfg.InferenceGRPCAddr)
	} else {
		log.Warn("INFERENCE_GRPC_ADDR no configurada: los reportes se guardarán sin sugerencia de la IA")
	}

	reporteRepo := repositories.NewReporteRepository(pool)
	categoriaRepo := repositories.NewCategoriaRepository(pool)
	deteccionRepo := repositories.NewDeteccionRepository(pool)
	sugerenciaSvc := services.NewSugerenciaService(detector, deteccionRepo, categoriaRepo, log)
	reporteSvc := services.NewReporteService(reporteRepo, categoriaRepo, archivoSvc, sugerenciaSvc, log)
	clasificacionRepo := repositories.NewClasificacionRepository(pool)
	clasificacionSvc := services.NewClasificacionService(clasificacionRepo, deteccionRepo, categoriaRepo, log)
	consultaRepo := repositories.NewConsultaReporteRepository(pool)
	consultaSvc := services.NewConsultaReporteService(consultaRepo, log)
	cercanosRepo := repositories.NewCercanosRepository(pool)
	cercanosSvc := services.NewCercanosService(cercanosRepo, dominio.ParametrosCercania{
		RadioM:      cfg.DuplicadosRadioM,
		VentanaDias: cfg.DuplicadosVentanaDias,
		Limite:      dominio.LimiteCercanosDefecto,
	}, log)

	r := router.New(log, tokens, router.Handlers{
		Health:        handlers.NewHealthHandler(healthSvc),
		Auth:          handlers.NewAuthHandler(authSvc),
		Usuario:       handlers.NewUsuarioHandler(usuarioSvc),
		Archivo:       handlers.NewArchivoHandler(archivoSvc),
		Reporte:       handlers.NewReporteHandler(reporteSvc),
		Clasificacion: handlers.NewClasificacionHandler(clasificacionSvc),
		Consulta:      handlers.NewConsultaReporteHandler(consultaSvc),
		Cercanos:      handlers.NewCercanosHandler(cercanosSvc),
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("API escuchando", "addr", srv.Addr, "arranque_ms", time.Since(inicio).Milliseconds())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("no se pudo iniciar el servidor HTTP: %w", err)
	case <-ctx.Done():
	}

	log.Info("apagando servidor...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("apagado forzado: %w", err)
	}
	log.Info("servidor detenido correctamente")
	return nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	opts := &slog.HandlerOptions{Level: level}
	if cfg.AppEnv == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
