package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
)

type HealthService struct {
	repo       *repositories.HealthRepository
	inferencia *InferenciaMonitor
	log        *slog.Logger
	version    string
	inicio     time.Time
	dbCaida    atomic.Bool
}

func NewHealthService(repo *repositories.HealthRepository, inferencia *InferenciaMonitor,
	log *slog.Logger, version string, inicio time.Time) *HealthService {
	return &HealthService{repo: repo, inferencia: inferencia, log: log, version: version, inicio: inicio}
}

func (s *HealthService) Check(ctx context.Context) models.HealthStatus {
	db := s.checkDB(ctx)
	inf := s.inferencia.Estado() // lectura instantánea, no hace red

	st := models.HealthStatus{
		Status:         models.EstadoOK,
		Version:        s.version,
		UptimeSegundos: int64(time.Since(s.inicio).Seconds()),
		Timestamp:      time.Now().UTC(),
		Dependencias:   models.Dependencias{Database: db, Inferencia: inf},
	}
	switch {
	case db.Estado != models.DependenciaActiva:
		st.Status = models.EstadoCaido
	case inf.Estado == models.DependenciaCaida:
		st.Status = models.EstadoDegradado
	}
	return st
}

func (s *HealthService) checkDB(ctx context.Context) models.Dependencia {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	inicio := time.Now()
	err := s.repo.Ping(ctx)
	d := models.Dependencia{Estado: models.DependenciaActiva, LatenciaMs: time.Since(inicio).Milliseconds()}

	if err != nil {
		d.Estado = models.DependenciaCaida
		if !s.dbCaida.Swap(true) { // solo la primera vez que cae
			s.log.Error("PostgreSQL no responde", "error", err)
		}
	} else if s.dbCaida.Swap(false) {
		s.log.Info("PostgreSQL se recuperó", "latencia_ms", d.LatenciaMs)
	}
	return d
}
