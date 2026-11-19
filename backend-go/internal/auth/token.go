package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

const emisor = "urbia-api"

var ErrTokenInvalido = errors.New("token inválido o expirado")

type Claims struct {
	Rol dominio.Rol `json:"rol"`
	jwt.RegisteredClaims
}

// Sesion es lo que queda en el contexto de Gin tras validar el token.
type Sesion struct {
	UsuarioID int64
	Rol       dominio.Rol
}

type TokenManager struct {
	secreto []byte
	ttl     time.Duration
}

func NewTokenManager(secreto string, ttl time.Duration) *TokenManager {
	return &TokenManager{secreto: []byte(secreto), ttl: ttl}
}

func (m *TokenManager) Generar(usuarioID int64, rol dominio.Rol) (string, time.Time, error) {
	ahora := time.Now()
	expira := ahora.Add(m.ttl)
	claims := Claims{
		Rol: rol,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(usuarioID, 10),
			Issuer:    emisor,
			IssuedAt:  jwt.NewNumericDate(ahora),
			ExpiresAt: jwt.NewNumericDate(expira),
		},
	}
	firmado, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secreto)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("firmar token: %w", err)
	}
	return firmado, expira, nil
}

func (m *TokenManager) Validar(tokenStr string) (Sesion, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims,
		func(*jwt.Token) (any, error) { return m.secreto, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // bloquea alg "none" y otros
		jwt.WithIssuer(emisor),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Sesion{}, ErrTokenInvalido
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return Sesion{}, ErrTokenInvalido
	}
	rol, err := dominio.ParseRol(string(claims.Rol))
	if err != nil {
		return Sesion{}, ErrTokenInvalido
	}
	return Sesion{UsuarioID: id, Rol: rol}, nil
}
