package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

func TestAutenticacionYAutorizacion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secreto := strings.Repeat("s", 32)
	tm := auth.NewTokenManager(secreto, time.Hour)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }

	r := gin.New()
	r.GET("/privada", RequireAuth(tm), ok)
	r.GET("/admin", RequireAuth(tm), RequireRol(dominio.RolAdministrador), ok)
	r.PATCH("/estado", RequireAuth(tm), RequirePermiso(dominio.Rol.PuedeCambiarEstado), ok)

	ciudadano, _, _ := tm.Generar(1, dominio.RolCiudadano)
	admin, _, _ := tm.Generar(2, dominio.RolAdministrador)
	expirado, _, _ := auth.NewTokenManager(secreto, -time.Minute).Generar(1, dominio.RolCiudadano)
	ajeno, _, _ := auth.NewTokenManager(strings.Repeat("x", 32), time.Hour).Generar(2, dominio.RolAdministrador)

	casos := []struct {
		nombre, metodo, ruta, token string
		esperado                    int
	}{
		{"sin token", http.MethodGet, "/privada", "", 401},
		{"token basura", http.MethodGet, "/privada", "abc.def.ghi", 401},
		{"token expirado", http.MethodGet, "/privada", expirado, 401},
		{"firmado con otro secreto", http.MethodGet, "/admin", ajeno, 401},
		{"ciudadano en ruta privada", http.MethodGet, "/privada", ciudadano, 200},
		{"ciudadano en ruta admin", http.MethodGet, "/admin", ciudadano, 403},
		{"admin en ruta admin", http.MethodGet, "/admin", admin, 200},
		{"ciudadano cambia estado", http.MethodPatch, "/estado", ciudadano, 403},
		{"admin cambia estado", http.MethodPatch, "/estado", admin, 200},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			req := httptest.NewRequest(tc.metodo, tc.ruta, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.esperado {
				t.Errorf("esperado %d, obtenido %d", tc.esperado, w.Code)
			}
		})
	}
}
