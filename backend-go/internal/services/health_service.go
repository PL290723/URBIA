package services

import (
	"context"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
)

type HealthService struct {
	repo *repositories.HealthRepository
}

func NewHealthService(repo *repositories.HealthRepository) *HealthService {
	return &HealthService{repo: repo}
}

func (s *HealthService) Check(ctx context.Context) models.HealthStatus {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	st := models.HealthStatus{Status: "ok", Database: "up", Timestamp: time.Now().UTC()}
	if err := s.repo.Ping(ctx); err != nil {
		st.Status, st.Database = "degraded", "down"
	}
	return st
}
