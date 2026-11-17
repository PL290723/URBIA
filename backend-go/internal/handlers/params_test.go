package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/middleware"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Pruebas de las utilidades de entrada de los handlers (HU 09.2).

func TestParseID(t *testing.T) {
	r := gin.New()
	r.GET("/r/:id", func(c *gin.Context) {
		if id, ok := parseID(c, "id"); ok {
			response.OK(c, gin.H{"id": id})
		}
	})

	validos := map[string]int64{"1": 1, "42": 42, "9223372036854775807": 9223372036854775807}
	for entrada, esperado := range validos {
		w := servir(r, peticion(http.MethodGet, "/r/"+entrada, "", ""))
		if w.Code != http.StatusOK || leer(t, w).Data["id"] != float64(esperado) {
			t.Errorf("%q: %d %s", entrada, w.Code, w.Body.String())
		}
	}
	for _, entrada := range []string{"0", "-5", "abc", "1.5", "9223372036854775808", "1%27%20OR%201=1"} {
		t.Run(entrada, func(t *testing.T) {
			esperarError(t, servir(r, peticion(http.MethodGet, "/r/"+entrada, "", "")),
				http.StatusBadRequest, response.CodeBadRequest)
		})
	}
}

func TestNombreArchivoValido(t *testing.T) {
	validos := []string{"foto.jpg", "IMG_2026-11-17 10.30.jpeg", "bache ñandú.png", strings.Repeat("a", 255)}
	for _, n := range validos {
		if !nombreArchivoValido(n) {
			t.Errorf("%q debería ser válido", n)
		}
	}
	invalidos := []string{
		"", ".", "..",
		"../../etc/passwd", `..\..\windows\win.ini`, "dir/foto.jpg",
		"foto.jpg:oculto", // flujo alterno de NTFS
		"foto\x00.jpg",    // byte nulo
		"foto\n.jpg",      // salto de línea (inyección en logs)
		"\xff\xfe.jpg",    // UTF-8 inválido
		strings.Repeat("a", 256),
	}
	for _, n := range invalidos {
		if nombreArchivoValido(n) {
			t.Errorf("%q debería rechazarse", n)
		}
	}
}

func TestSesionRequerida(t *testing.T) {
	r := gin.New()
	handler := func(c *gin.Context) {
		if s, ok := sesionRequerida(c); ok {
			response.OK(c, gin.H{"id": s.UsuarioID})
		}
	}
	r.GET("/sin-middleware", handler)
	r.GET("/con-middleware", middleware.RequireAuth(tokensPrueba), handler)

	// Si una ruta olvidara RequireAuth, el handler igual responde 401 (defensa en profundidad).
	esperarError(t, servir(r, peticion(http.MethodGet, "/sin-middleware", "", "")),
		http.StatusUnauthorized, response.CodeUnauthorized)

	w := servir(r, peticion(http.MethodGet, "/con-middleware", "", tokenDe(t, 5, "ciudadano")))
	if w.Code != http.StatusOK || leer(t, w).Data["id"] != float64(5) {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
}

func TestBindJSON(t *testing.T) {
	type entrada struct {
		Correo string `json:"correo" binding:"required,email"`
		Edad   int    `json:"edad" binding:"min=1"`
	}
	r := gin.New()
	r.Use(middleware.LimiteCuerpo())
	r.POST("/", func(c *gin.Context) {
		var e entrada
		if bindJSON(c, &e) {
			c.JSON(http.StatusOK, gin.H{"correo": e.Correo})
		}
	})

	t.Run("válido", func(t *testing.T) {
		w := servir(r, peticion(http.MethodPost, "/", `{"correo":"a@b.mx","edad":3}`, ""))
		if w.Code != http.StatusOK {
			t.Errorf("%d %s", w.Code, w.Body.String())
		}
	})
	t.Run("JSON mal formado", func(t *testing.T) {
		w := servir(r, peticion(http.MethodPost, "/", `{"correo": "a@b.mx",`, ""))
		esperarError(t, w, http.StatusBadRequest, response.CodeBadRequest)
	})
	t.Run("tipo equivocado", func(t *testing.T) {
		w := servir(r, peticion(http.MethodPost, "/", `{"correo":"a@b.mx","edad":"tres"}`, ""))
		esperarError(t, w, http.StatusBadRequest, response.CodeBadRequest)
	})
	t.Run("reglas de validación con nombre JSON del campo", func(t *testing.T) {
		w := servir(r, peticion(http.MethodPost, "/", `{"correo":"no-es-correo","edad":0}`, ""))
		res := esperarError(t, w, http.StatusBadRequest, response.CodeValidation)
		campos := strings.Join(camposConError(t, res), ",")
		if campos != "correo,edad" {
			t.Errorf("details debe usar los nombres JSON: %q", campos)
		}
	})
	t.Run("cuerpo de más de 64 KiB", func(t *testing.T) {
		grande := `{"correo":"` + strings.Repeat("a", 70<<10) + `@b.mx"}`
		w := servir(r, peticion(http.MethodPost, "/", grande, ""))
		esperarError(t, w, http.StatusRequestEntityTooLarge, response.CodeBodyTooLarge)
	})
	t.Run("cuerpo grande sin Content-Length", func(t *testing.T) {
		req := peticion(http.MethodPost, "/", `{"correo":"`+strings.Repeat("a", 70<<10)+`@b.mx"}`, "")
		req.ContentLength = -1
		w := servir(r, req)
		esperarError(t, w, http.StatusRequestEntityTooLarge, response.CodeBodyTooLarge)
	})
}
