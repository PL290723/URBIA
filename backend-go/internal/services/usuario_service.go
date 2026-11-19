package services

import (
	"context"

	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
)

type UsuarioService struct {
	repo *repositories.UsuarioRepository
}

func NewUsuarioService(repo *repositories.UsuarioRepository) *UsuarioService {
	return &UsuarioService{repo: repo}
}

func (s *UsuarioService) Listar(ctx context.Context) ([]models.Usuario, error) {
	return s.repo.Listar(ctx)
}
