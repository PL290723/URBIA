package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

var secreto = strings.Repeat("s", 32)

func TestHashYComparar(t *testing.T) {
	hash, err := HashContrasena("secreta123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "secreta123" {
		t.Fatal("la contraseña no debe quedar en texto plano")
	}
	if CompararContrasena(hash, "secreta123") != nil {
		t.Error("la contraseña correcta debería coincidir")
	}
	if CompararContrasena(hash, "otra") == nil {
		t.Error("una contraseña incorrecta no debería coincidir")
	}
}

func TestTokenValido(t *testing.T) {
	tm := NewTokenManager(secreto, time.Hour)
	tok, _, err := tm.Generar(7, dominio.RolAdministrador)
	if err != nil {
		t.Fatal(err)
	}
	s, err := tm.Validar(tok)
	if err != nil || s.UsuarioID != 7 || s.Rol != dominio.RolAdministrador {
		t.Fatalf("sesión inesperada: %+v, err=%v", s, err)
	}
	if _, err := tm.Validar(tok + "x"); err == nil {
		t.Error("un token alterado debería rechazarse")
	}
}

func TestTokenExpirado(t *testing.T) {
	tok, _, _ := NewTokenManager(secreto, -time.Minute).Generar(1, dominio.RolCiudadano)
	if _, err := NewTokenManager(secreto, time.Hour).Validar(tok); err == nil {
		t.Fatal("un token expirado debería rechazarse")
	}
}

func TestTokenAlgNone(t *testing.T) {
	claims := Claims{Rol: dominio.RolAdministrador, RegisteredClaims: jwt.RegisteredClaims{
		Subject: "1", Issuer: emisor, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := NewTokenManager(secreto, time.Hour).Validar(tok); err == nil {
		t.Fatal("un token con alg none debería rechazarse")
	}
}
