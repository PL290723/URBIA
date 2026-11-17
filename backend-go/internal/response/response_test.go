package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Pruebas del formato único de respuesta y de la traducción error → HTTP (HU 09.2).

func init() { gin.SetMode(gin.TestMode) }

// ejecutar corre un handler en un contexto de Gin y devuelve la respuesta grabada.
func ejecutar(handler func(c *gin.Context)) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler(c)
	return w, c
}

func leerCuerpo(t *testing.T, w *httptest.ResponseRecorder) Body {
	t.Helper()
	var b Body
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("la respuesta no es JSON: %v (%q)", err, w.Body.String())
	}
	return b
}

func TestConstructoresDeAppError(t *testing.T) {
	casos := []struct {
		nombre string
		err    *AppError
		status int
		code   string
	}{
		{"BadRequest", BadRequest("x"), http.StatusBadRequest, CodeBadRequest},
		{"Unauthorized", Unauthorized("x"), http.StatusUnauthorized, CodeUnauthorized},
		{"Forbidden", Forbidden("x"), http.StatusForbidden, CodeForbidden},
		{"NotFound", NotFound("x"), http.StatusNotFound, CodeNotFound},
		{"Conflict", Conflict("x"), http.StatusConflict, CodeConflict},
		{"Validation", Validation([]string{"campo"}), http.StatusBadRequest, CodeValidation},
		{"PayloadTooLarge", PayloadTooLarge("x"), http.StatusRequestEntityTooLarge, CodePayloadTooLarge},
		{"BodyTooLarge", BodyTooLarge("x"), http.StatusRequestEntityTooLarge, CodeBodyTooLarge},
		{"UnsupportedMediaType", UnsupportedMediaType("x"), http.StatusUnsupportedMediaType, CodeUnsupportedMedia},
		{"UnprocessableImage", UnprocessableImage("x"), http.StatusUnprocessableEntity, CodeInvalidImage},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			if tc.err.Status != tc.status || tc.err.Code != tc.code {
				t.Errorf("obtenido %d %s, esperado %d %s", tc.err.Status, tc.err.Code, tc.status, tc.code)
			}
			if tc.err.Error() != tc.err.Message {
				t.Error("Error() debe devolver el mensaje para el usuario")
			}
		})
	}
}

func TestHandleErrorConAppError(t *testing.T) {
	detalles := []map[string]string{{"campo": "descripcion", "regla": "longitud"}}
	w, _ := ejecutar(func(c *gin.Context) { HandleError(c, Validation(detalles)) })

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, esperado 400", w.Code)
	}
	b := leerCuerpo(t, w)
	if b.Success || b.Error == nil || b.Error.Code != CodeValidation || b.Data != nil {
		t.Fatalf("cuerpo inesperado: %+v", b)
	}
	if !strings.Contains(w.Body.String(), `"campo":"descripcion"`) {
		t.Errorf("details debe llegar al cliente: %s", w.Body.String())
	}
}

// Un AppError envuelto (fmt.Errorf con %w) se sigue reconociendo.
func TestHandleErrorConAppErrorEnvuelto(t *testing.T) {
	err := fmt.Errorf("capa de servicio: %w", NotFound("Reporte no encontrado"))
	w, _ := ejecutar(func(c *gin.Context) { HandleError(c, err) })
	if w.Code != http.StatusNotFound || leerCuerpo(t, w).Error.Message != "Reporte no encontrado" {
		t.Errorf("status = %d, cuerpo = %s", w.Code, w.Body.String())
	}
}

// Un error cualquiera (p. ej. de PostgreSQL) es 500 y NO filtra el detalle interno.
func TestHandleErrorInternoNoFiltraDetalles(t *testing.T) {
	interno := errors.New(`pq: password authentication failed for user "urbia_app" at 10.0.0.5:5432`)
	w, c := ejecutar(func(c *gin.Context) { HandleError(c, interno) })

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, esperado 500", w.Code)
	}
	b := leerCuerpo(t, w)
	if b.Error.Code != CodeInternal || b.Error.Message != "Ocurrió un error inesperado" {
		t.Errorf("cuerpo inesperado: %+v", b.Error)
	}
	for _, secreto := range []string{"urbia_app", "5432", "password"} {
		if strings.Contains(w.Body.String(), secreto) {
			t.Errorf("la respuesta filtra %q: %s", secreto, w.Body.String())
		}
	}
	if len(c.Errors) != 1 || !errors.Is(c.Errors[0].Err, interno) {
		t.Error("el error real debe quedar en c.Errors para el log")
	}
	if !c.IsAborted() {
		t.Error("HandleError debe abortar la cadena de handlers")
	}
}

func TestRespuestasExitosas(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		w, _ := ejecutar(func(c *gin.Context) { OK(c, gin.H{"id": 7}) })
		if w.Code != http.StatusOK || w.Body.String() != `{"success":true,"data":{"id":7}}` {
			t.Errorf("%d %s", w.Code, w.Body.String())
		}
	})
	t.Run("Created", func(t *testing.T) {
		w, _ := ejecutar(func(c *gin.Context) { Created(c, gin.H{"id": 8}) })
		if w.Code != http.StatusCreated || !leerCuerpo(t, w).Success {
			t.Errorf("%d %s", w.Code, w.Body.String())
		}
	})
	t.Run("NoContent", func(t *testing.T) {
		w, c := ejecutar(func(c *gin.Context) { NoContent(c) })
		c.Writer.WriteHeaderNow()
		if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
			t.Errorf("%d %q", w.Code, w.Body.String())
		}
	})
}

func TestErrorYValidationError(t *testing.T) {
	w, _ := ejecutar(func(c *gin.Context) { Error(c, http.StatusNotFound, CodeNotFound, "Ruta no encontrada") })
	if w.Body.String() != `{"success":false,"error":{"code":"NO_ENCONTRADO","message":"Ruta no encontrada"}}` {
		t.Errorf("formato de error inesperado: %s", w.Body.String())
	}

	w, _ = ejecutar(func(c *gin.Context) { ValidationError(c, []string{"correo"}) })
	b := leerCuerpo(t, w)
	if w.Code != http.StatusBadRequest || b.Error.Code != CodeValidation || b.Error.Details == nil {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
}

func TestUnavailableIncluyeDatos(t *testing.T) {
	w, _ := ejecutar(func(c *gin.Context) { Unavailable(c, gin.H{"status": "caido"}, "La base de datos no responde") })
	b := leerCuerpo(t, w)
	if w.Code != http.StatusServiceUnavailable || b.Success || b.Data == nil || b.Error.Code != CodeServiceUnavailable {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
}
