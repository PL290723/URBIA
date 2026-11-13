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

	r := router.New(log, tokens, router.Handlers{
		Health:  handlers.NewHealthHandler(healthSvc),
		Auth:    handlers.NewAuthHandler(authSvc),
		Usuario: handlers.NewUsuarioHandler(usuarioSvc),
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
