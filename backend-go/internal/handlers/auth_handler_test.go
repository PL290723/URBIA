package handlers

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

// Pruebas HTTP de /auth: handler + service reales, repositorio en memoria (HU 09.2).

func routerAuth(t *testing.T, repo *usuariosEnMemoria) *gin.Engine {
	t.Helper()
	svc, err := services.NewAuthService(repo, tokensPrueba)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(svc)
	r := gin.New()
	r.Use(middleware.LimiteCuerpo())
	r.POST("/auth/registro", h.Registro)
	r.POST("/auth/login", h.Login)
	r.GET("/auth/perfil", middleware.RequireAuth(tokensPrueba), h.Perfil)
	return r
}

func nuevoRepo() *usuariosEnMemoria {
	return &usuariosEnMemoria{porCorreo: map[string]*models.Usuario{}}
}

const registroAna = `{"nombre":"Ana López","correo":"ana@ejemplo.com","contrasena":"Segura091!"}`

func TestRegistroHTTP(t *testing.T) {
	r := routerAuth(t, nuevoRepo())

	w := servir(r, peticion(http.MethodPost, "/auth/registro", registroAna, ""))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	res := leer(t, w)
	usuario := res.Data["usuario"].(map[string]any)
	if res.Data["token"] == "" || res.Data["tipo_token"] != "Bearer" || usuario["rol"] != "ciudadano" {
		t.Errorf("respuesta inesperada: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "hash") || strings.Contains(w.Body.String(), "Segura091!") {
		t.Error("la respuesta no debe incluir la contraseña ni su hash")
	}

	// Mismo correo con otras mayúsculas → 409
	w = servir(r, peticion(http.MethodPost, "/auth/registro",
		`{"nombre":"Otra","correo":"ANA@ejemplo.com","contrasena":"OtraClave1"}`, ""))
	esperarError(t, w, http.StatusConflict, response.CodeConflict)
}

func TestRegistroHTTPValidaciones(t *testing.T) {
	r := routerAuth(t, nuevoRepo())
	casos := []struct {
		nombre, cuerpo string
		status         int
		code           string
		campos         string
	}{
		{"sin cuerpo", "", http.StatusBadRequest, response.CodeBadRequest, ""},
		{"JSON roto", `{"nombre":`, http.StatusBadRequest, response.CodeBadRequest, ""},
		{"faltan campos", `{}`, http.StatusBadRequest, response.CodeValidation, "nombre,correo,contrasena"},
		{"correo inválido", `{"nombre":"A","correo":"ana","contrasena":"Segura091!"}`, http.StatusBadRequest, response.CodeValidation, "correo"},
		{"contraseña corta", `{"nombre":"A","correo":"a@b.mx","contrasena":"1234567"}`, http.StatusBadRequest, response.CodeValidation, "contrasena"},
		{"nombre de solo espacios", `{"nombre":"   ","correo":"a@b.mx","contrasena":"Segura091!"}`, http.StatusBadRequest, response.CodeBadRequest, ""},
		{"intenta registrarse como admin (se ignora el rol)", `{"nombre":"A","correo":"x@b.mx","contrasena":"Segura091!","rol":"administrador"}`, http.StatusCreated, "", ""},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			w := servir(r, peticion(http.MethodPost, "/auth/registro", tc.cuerpo, ""))
			if tc.status == http.StatusCreated {
				if w.Code != http.StatusCreated || leer(t, w).Data["usuario"].(map[string]any)["rol"] != "ciudadano" {
					t.Errorf("%d %s", w.Code, w.Body.String())
				}
				return
			}
			res := esperarError(t, w, tc.status, tc.code)
			if tc.campos != "" {
				if got := strings.Join(camposConError(t, res), ","); got != tc.campos {
					t.Errorf("campos con error = %q, esperado %q", got, tc.campos)
				}
			}
		})
	}
}

func TestLoginHTTP(t *testing.T) {
	r := routerAuth(t, nuevoRepo())
	servir(r, peticion(http.MethodPost, "/auth/registro", registroAna, ""))

	w := servir(r, peticion(http.MethodPost, "/auth/login", `{"correo":"ana@ejemplo.com","contrasena":"Segura091!"}`, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	token := leer(t, w).Data["token"].(string)

	// El token sirve para /perfil
	w = servir(r, peticion(http.MethodGet, "/auth/perfil", "", token))
	if w.Code != http.StatusOK || leer(t, w).Data["correo"] != "ana@ejemplo.com" {
		t.Errorf("perfil: %d %s", w.Code, w.Body.String())
	}

	// Contraseña incorrecta y correo inexistente: mismo cuerpo exacto
	malClave := servir(r, peticion(http.MethodPost, "/auth/login", `{"correo":"ana@ejemplo.com","contrasena":"otra"}`, ""))
	malCorreo := servir(r, peticion(http.MethodPost, "/auth/login", `{"correo":"nadie@ejemplo.com","contrasena":"Segura091!"}`, ""))
	esperarError(t, malClave, http.StatusUnauthorized, response.CodeUnauthorized)
	esperarError(t, malCorreo, http.StatusUnauthorized, response.CodeUnauthorized)
	if malClave.Body.String() != malCorreo.Body.String() {
		t.Errorf("las respuestas revelan si el correo existe:\n%s\n%s", malClave.Body.String(), malCorreo.Body.String())
	}
}

func TestPerfilHTTPSinTokenOTokenInvalido(t *testing.T) {
	r := routerAuth(t, nuevoRepo())
	for nombre, token := range map[string]string{"sin token": "", "basura": "abc.def.ghi"} {
		t.Run(nombre, func(t *testing.T) {
			w := servir(r, peticion(http.MethodGet, "/auth/perfil", "", token))
			esperarError(t, w, http.StatusUnauthorized, response.CodeUnauthorized)
			if w.Header().Get("WWW-Authenticate") != `Bearer realm="urbia"` {
				t.Error("falta WWW-Authenticate")
			}
		})
	}
	// Token válido de un usuario borrado
	w := servir(r, peticion(http.MethodGet, "/auth/perfil", "", tokenDe(t, 999, "ciudadano")))
	esperarError(t, w, http.StatusUnauthorized, response.CodeUnauthorized)
}

// Error de repositorio → 500 genérico, sin filtrar el detalle de la BD.
func TestAuthHTTPErrorDeRepositorio(t *testing.T) {
	repo := nuevoRepo()
	repo.falla = errors.New(`dial tcp 127.0.0.1:5432: connect: connection refused (usuario urbia_app)`)
	r := routerAuth(t, repo)

	for _, req := range []*http.Request{
		peticion(http.MethodPost, "/auth/registro", registroAna, ""),
		peticion(http.MethodPost, "/auth/login", `{"correo":"ana@ejemplo.com","contrasena":"Segura091!"}`, ""),
		peticion(http.MethodGet, "/auth/perfil", "", tokenDe(t, 1, "ciudadano")),
	} {
		t.Run(req.Method+" "+req.URL.Path, func(t *testing.T) {
			w := servir(r, req)
			res := esperarError(t, w, http.StatusInternalServerError, response.CodeInternal)
			if res.Error.Message != "Ocurrió un error inesperado" ||
				strings.Contains(w.Body.String(), "5432") || strings.Contains(w.Body.String(), "urbia_app") {
				t.Errorf("filtra detalles internos: %s", w.Body.String())
			}
		})
	}
}
