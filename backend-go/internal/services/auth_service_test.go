package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Pruebas unitarias de AuthService con un repositorio en memoria (HU 09.2).

// usuariosEnMemoria es un doble de RepositorioUsuarios.
// Si falla != nil, todas las operaciones devuelven ese error (BD caída).
type usuariosEnMemoria struct {
	porCorreo map[string]*models.Usuario
	siguiente int64
	falla     error
}

func nuevoRepoUsuarios() *usuariosEnMemoria {
	return &usuariosEnMemoria{porCorreo: map[string]*models.Usuario{}, siguiente: 1}
}

func (r *usuariosEnMemoria) Crear(_ context.Context, u *models.Usuario) error {
	if r.falla != nil {
		return r.falla
	}
	if _, existe := r.porCorreo[u.Correo]; existe {
		return repositories.ErrCorreoDuplicado
	}
	u.ID = r.siguiente
	r.siguiente++
	copia := *u
	r.porCorreo[u.Correo] = &copia
	return nil
}

func (r *usuariosEnMemoria) BuscarPorCorreo(_ context.Context, correo string) (*models.Usuario, error) {
	if r.falla != nil {
		return nil, r.falla
	}
	u, ok := r.porCorreo[correo]
	if !ok {
		return nil, repositories.ErrNoEncontrado
	}
	return u, nil
}

func (r *usuariosEnMemoria) BuscarPorID(_ context.Context, id int64) (*models.Usuario, error) {
	if r.falla != nil {
		return nil, r.falla
	}
	for _, u := range r.porCorreo {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, repositories.ErrNoEncontrado
}

var tokensPrueba = auth.NewTokenManager(strings.Repeat("s", 32), time.Hour)

func nuevoAuthService(t *testing.T, repo RepositorioUsuarios) *AuthService {
	t.Helper()
	svc, err := NewAuthService(repo, tokensPrueba)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func esperarAppError(t *testing.T, err error, status int, code string) *response.AppError {
	t.Helper()
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("se esperaba AppError %d %s, llegó %v", status, code, err)
	}
	if appErr.Status != status || appErr.Code != code {
		t.Fatalf("obtenido %d %s, esperado %d %s", appErr.Status, appErr.Code, status, code)
	}
	return appErr
}

func registrar(t *testing.T, svc *AuthService, correo, clave string) *models.AuthResponse {
	t.Helper()
	res, err := svc.Registrar(context.Background(), models.RegistroRequest{Nombre: "Ana", Correo: correo, Contrasena: clave})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestRegistrarCreaCiudadanoConHashYToken(t *testing.T) {
	repo := nuevoRepoUsuarios()
	res := registrar(t, nuevoAuthService(t, repo), "  Ana@Ejemplo.COM ", "Segura091!")

	guardado := repo.porCorreo["ana@ejemplo.com"]
	if guardado == nil {
		t.Fatal("el correo debe guardarse normalizado (minúsculas y sin espacios)")
	}
	if guardado.Rol != dominio.RolCiudadano {
		t.Errorf("el registro público solo crea ciudadanos, creó %q", guardado.Rol)
	}
	if guardado.HashContrasena == "Segura091!" || auth.CompararContrasena(guardado.HashContrasena, "Segura091!") != nil {
		t.Error("la contraseña debe guardarse como hash bcrypt")
	}
	sesion, err := tokensPrueba.Validar(res.Token)
	if err != nil || sesion.UsuarioID != guardado.ID || sesion.Rol != dominio.RolCiudadano {
		t.Errorf("token inválido: %+v %v", sesion, err)
	}
	if res.TipoToken != "Bearer" {
		t.Errorf("tipo_token = %q", res.TipoToken)
	}
}

func TestRegistrarCorreoDuplicadoEs409(t *testing.T) {
	svc := nuevoAuthService(t, nuevoRepoUsuarios())
	registrar(t, svc, "ana@ejemplo.com", "Segura091!")
	_, err := svc.Registrar(context.Background(), models.RegistroRequest{Nombre: "Otra", Correo: "ANA@ejemplo.com", Contrasena: "OtraClave1"})
	esperarAppError(t, err, http.StatusConflict, response.CodeConflict)
}

func TestRegistrarValidaNombreYContrasena(t *testing.T) {
	svc := nuevoAuthService(t, nuevoRepoUsuarios())
	_, err := svc.Registrar(context.Background(), models.RegistroRequest{Nombre: "   ", Correo: "a@b.mx", Contrasena: "Segura091!"})
	esperarAppError(t, err, http.StatusBadRequest, response.CodeBadRequest)

	// 37 "ñ" = 74 bytes: pasa el max=72 de caracteres del binding, pero bcrypt solo usa 72 bytes
	_, err = svc.Registrar(context.Background(), models.RegistroRequest{Nombre: "Ana", Correo: "a@b.mx", Contrasena: strings.Repeat("ñ", 37)})
	esperarAppError(t, err, http.StatusBadRequest, response.CodeBadRequest)
}

// Un error de la BD NO se disfraza de error del usuario: sube tal cual y el handler responde 500.
func TestRegistrarErrorDeRepositorioNoEsAppError(t *testing.T) {
	repo := nuevoRepoUsuarios()
	repo.falla = errors.New("conexión rechazada")
	_, err := nuevoAuthService(t, repo).Registrar(context.Background(),
		models.RegistroRequest{Nombre: "Ana", Correo: "a@b.mx", Contrasena: "Segura091!"})

	var appErr *response.AppError
	if err == nil || errors.As(err, &appErr) {
		t.Fatalf("se esperaba un error interno (→ 500), llegó %v", err)
	}
}

func TestIniciarSesion(t *testing.T) {
	repo := nuevoRepoUsuarios()
	svc := nuevoAuthService(t, repo)
	registrar(t, svc, "ana@ejemplo.com", "Segura091!")

	t.Run("correcto, con correo en otro formato", func(t *testing.T) {
		res, err := svc.IniciarSesion(context.Background(), models.LoginRequest{Correo: " ANA@ejemplo.com", Contrasena: "Segura091!"})
		if err != nil || res.Token == "" || res.Usuario.Correo != "ana@ejemplo.com" {
			t.Fatalf("%+v %v", res, err)
		}
	})

	// Mismo 401 y mismo mensaje: no revela qué correos existen.
	_, errClave := svc.IniciarSesion(context.Background(), models.LoginRequest{Correo: "ana@ejemplo.com", Contrasena: "incorrecta"})
	_, errCorreo := svc.IniciarSesion(context.Background(), models.LoginRequest{Correo: "nadie@ejemplo.com", Contrasena: "Segura091!"})
	a := esperarAppError(t, errClave, http.StatusUnauthorized, response.CodeUnauthorized)
	b := esperarAppError(t, errCorreo, http.StatusUnauthorized, response.CodeUnauthorized)
	if a.Message != b.Message {
		t.Errorf("los mensajes deben ser iguales: %q vs %q", a.Message, b.Message)
	}
}

// Con la BD caída el login NO debe responder 401 (sería un "contraseña incorrecta" falso).
func TestIniciarSesionConBDCaida(t *testing.T) {
	repo := nuevoRepoUsuarios()
	repo.falla = errors.New("timeout")
	_, err := nuevoAuthService(t, repo).IniciarSesion(context.Background(), models.LoginRequest{Correo: "a@b.mx", Contrasena: "x"})
	var appErr *response.AppError
	if err == nil || errors.As(err, &appErr) {
		t.Fatalf("se esperaba error interno, llegó %v", err)
	}
}

func TestPerfil(t *testing.T) {
	repo := nuevoRepoUsuarios()
	svc := nuevoAuthService(t, repo)
	res := registrar(t, svc, "ana@ejemplo.com", "Segura091!")

	u, err := svc.Perfil(context.Background(), res.Usuario.ID)
	if err != nil || u.Correo != "ana@ejemplo.com" {
		t.Fatalf("%+v %v", u, err)
	}
	// Token válido de un usuario que ya no existe → sesión inválida
	_, err = svc.Perfil(context.Background(), 9999)
	esperarAppError(t, err, http.StatusUnauthorized, response.CodeUnauthorized)
}
