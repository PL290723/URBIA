package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Mismo mensaje para correo inexistente y contraseña incorrecta: no revela si la cuenta existe.
const msgCredenciales = "Correo o contraseña incorrectos"

type AuthService struct {
	usuarios  *repositories.UsuarioRepository
	tokens    *auth.TokenManager
	dummyHash string
}

func NewAuthService(usuarios *repositories.UsuarioRepository, tokens *auth.TokenManager) (*AuthService, error) {
	dummy, err := auth.HashContrasena("contrasena-ficticia-para-tiempo-constante")
	if err != nil {
		return nil, err
	}
	return &AuthService{usuarios: usuarios, tokens: tokens, dummyHash: dummy}, nil
}

func (s *AuthService) Registrar(ctx context.Context, req models.RegistroRequest) (*models.AuthResponse, error) {
	nombre := strings.TrimSpace(req.Nombre)
	if nombre == "" {
		return nil, response.BadRequest("El nombre no puede estar vacío")
	}
	if len(req.Contrasena) > 72 { // límite de bcrypt en bytes (acentos ocupan 2)
		return nil, response.BadRequest("La contraseña es demasiado larga")
	}

	hash, err := auth.HashContrasena(req.Contrasena)
	if err != nil {
		return nil, fmt.Errorf("hash de contraseña: %w", err)
	}

	u := &models.Usuario{
		Nombre:         nombre,
		Correo:         normalizarCorreo(req.Correo),
		HashContrasena: hash,
		Rol:            dominio.RolCiudadano, // el registro público nunca crea administradores
	}
	if err := s.usuarios.Crear(ctx, u); err != nil {
		if errors.Is(err, repositories.ErrCorreoDuplicado) {
			return nil, response.Conflict("El correo ya está registrado")
		}
		return nil, err
	}
	return s.emitir(u)
}

func (s *AuthService) IniciarSesion(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	u, err := s.usuarios.BuscarPorCorreo(ctx, normalizarCorreo(req.Correo))
	if errors.Is(err, repositories.ErrNoEncontrado) {
		_ = auth.CompararContrasena(s.dummyHash, req.Contrasena) // iguala el tiempo de respuesta
		return nil, response.Unauthorized(msgCredenciales)
	}
	if err != nil {
		return nil, err
	}
	if err := auth.CompararContrasena(u.HashContrasena, req.Contrasena); err != nil {
		return nil, response.Unauthorized(msgCredenciales)
	}
	return s.emitir(u)
}

func (s *AuthService) Perfil(ctx context.Context, id int64) (*models.Usuario, error) {
	u, err := s.usuarios.BuscarPorID(ctx, id)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, response.Unauthorized("Sesión inválida")
	}
	return u, err
}

func (s *AuthService) emitir(u *models.Usuario) (*models.AuthResponse, error) {
	token, expira, err := s.tokens.Generar(u.ID, u.Rol)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{Token: token, TipoToken: "Bearer", ExpiraEn: expira, Usuario: *u}, nil
}

func normalizarCorreo(c string) string { return strings.ToLower(strings.TrimSpace(c)) }
