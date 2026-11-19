package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Pruebas HTTP de la validación de POST /reportes (HU 09.2). Todas fallan ANTES
// de llegar al service, por eso el handler se arma con un service nil: si alguna
// validación dejara pasar la petición, el panic lo convertiría en 500 y la prueba fallaría.

func routerReporte() *gin.Engine {
	h := NewReporteHandler(nil)
	r := gin.New()
	r.Use(middleware.Recovery(logMudo), middleware.LimiteCuerpo())
	r.POST("/reportes", middleware.RequireAuth(tokensPrueba),
		middleware.RequirePermiso(dominio.Rol.PuedeCrearReporte), h.Crear)
	return r
}

func enviarReporte(t *testing.T, campos map[string]string, nombreArchivo string, archivo []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	cuerpo, tipo := multipartForm(t, campos, nombreArchivo, archivo)
	req := httptest.NewRequest(http.MethodPost, "/reportes", cuerpo)
	req.Header.Set("Content-Type", tipo)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return servir(routerReporte(), req)
}

func camposValidos() map[string]string {
	return map[string]string{
		"descripcion":      "Bache profundo frente a la escuela primaria",
		"latitud":          "20.522345",
		"longitud":         "-100.812345",
		"origen_ubicacion": "manual",
	}
}

func TestCrearReporteHTTPPermisos(t *testing.T) {
	esperarError(t, enviarReporte(t, camposValidos(), "foto.jpg", []byte("x"), ""),
		http.StatusUnauthorized, response.CodeUnauthorized)
	esperarError(t, enviarReporte(t, camposValidos(), "foto.jpg", []byte("x"), tokenDe(t, 1, dominio.RolAdministrador)),
		http.StatusForbidden, response.CodeForbidden)
}

func TestCrearReporteHTTPValidaciones(t *testing.T) {
	token := tokenDe(t, 2, dominio.RolCiudadano)
	con := func(cambios map[string]string) map[string]string {
		c := camposValidos()
		for k, v := range cambios {
			if v == "" {
				delete(c, k)
			} else {
				c[k] = v
			}
		}
		return c
	}

	casos := []struct {
		nombre        string
		campos        map[string]string
		archivo       string
		camposEsperan []string
	}{
		{"sin foto", camposValidos(), "", []string{"foto"}},
		{"formulario vacío", map[string]string{}, "", []string{"descripcion", "ubicacion", "foto"}},
		{"descripción corta", con(map[string]string{"descripcion": "Bache"}), "foto.jpg", []string{"descripcion"}},
		{"latitud fuera de rango", con(map[string]string{"latitud": "95"}), "foto.jpg", []string{"latitud"}},
		{"longitud no numérica", con(map[string]string{"longitud": "abc"}), "foto.jpg", []string{"longitud"}},
		{"coordenadas (0,0) sin señal", con(map[string]string{"latitud": "0", "longitud": "0"}), "foto.jpg", []string{"ubicacion"}},
		{"origen inválido", con(map[string]string{"origen_ubicacion": "wifi"}), "foto.jpg", []string{"origen_ubicacion"}},
		{"gps sin precisión", con(map[string]string{"origen_ubicacion": "gps"}), "foto.jpg", []string{"precision_m"}},
		{"categoría inválida", con(map[string]string{"id_categoria": "-1"}), "foto.jpg", []string{"id_categoria"}},
		{"nombre de archivo con flujo NTFS", camposValidos(), "foto.jpg:oculto", []string{"foto"}},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			w := enviarReporte(t, tc.campos, tc.archivo, []byte("contenido"), token)
			res := esperarError(t, w, http.StatusBadRequest, response.CodeValidation)
			obtenidos := camposConError(t, res)
			for _, c := range tc.camposEsperan {
				if !slices.Contains(obtenidos, c) {
					t.Errorf("falta el error del campo %q; llegaron %v", c, obtenidos)
				}
			}
		})
	}
}

// Todos los errores de campo llegan juntos, para corregirlos de una vez.
func TestCrearReporteHTTPReportaTodosLosErrores(t *testing.T) {
	w := enviarReporte(t, map[string]string{"descripcion": "x", "latitud": "200", "longitud": "20",
		"origen_ubicacion": "otro"}, "", nil, tokenDe(t, 2, dominio.RolCiudadano))
	res := esperarError(t, w, http.StatusBadRequest, response.CodeValidation)
	if n := len(camposConError(t, res)); n < 4 {
		t.Errorf("se esperaban al menos 4 errores (descripcion, latitud, origen, foto), llegaron %d: %v", n, res.Error.Details)
	}
}

func TestCrearReporteHTTPNoMultipart(t *testing.T) {
	req := peticion(http.MethodPost, "/reportes", `{"descripcion":"json no permitido"}`, tokenDe(t, 2, dominio.RolCiudadano))
	esperarError(t, servir(routerReporte(), req), http.StatusBadRequest, response.CodeBadRequest)
}

func TestCrearReporteHTTPDemasiadoGrande(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/reportes", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	req.Header.Set("Authorization", "Bearer "+tokenDe(t, 2, dominio.RolCiudadano))
	req.ContentLength = dominio.TamanoMaximoArchivo + 2<<20 // declara 12 MB
	w := servir(routerReporte(), req)
	res := esperarError(t, w, http.StatusRequestEntityTooLarge, response.CodePayloadTooLarge)
	if !strings.Contains(res.Error.Message, "10 MB") {
		t.Errorf("mensaje = %q", res.Error.Message)
	}
}
