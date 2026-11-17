package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Utilidades compartidas por las pruebas de handlers (HU 09.2): un router de Gin
// real, tokens firmados de prueba y dobles en memoria de los repositorios.

var (
	tokensPrueba = auth.NewTokenManager(strings.Repeat("h", 32), time.Hour)
	logMudo      = slog.New(slog.NewTextHandler(io.Discard, nil))
)

func init() { gin.SetMode(gin.TestMode) }

func tokenDe(t *testing.T, id int64, rol dominio.Rol) string {
	t.Helper()
	tok, _, err := tokensPrueba.Generar(id, rol)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// peticion arma una petición JSON (cuerpo "" = sin cuerpo) con token opcional.
func peticion(metodo, ruta, cuerpo, token string) *http.Request {
	var r io.Reader
	if cuerpo != "" {
		r = strings.NewReader(cuerpo)
	}
	req := httptest.NewRequest(metodo, ruta, r)
	if cuerpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func servir(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// respuesta decodifica el sobre {"success","data","error"}; data queda como mapa.
type respuesta struct {
	Success bool                `json:"success"`
	Data    map[string]any      `json:"data"`
	Error   *response.ErrorBody `json:"error"`
}

func leer(t *testing.T, w *httptest.ResponseRecorder) respuesta {
	t.Helper()
	var r respuesta
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("la respuesta no es JSON: %v (%q)", err, w.Body.String())
	}
	return r
}

// esperarError comprueba status y code de una respuesta de error.
func esperarError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) respuesta {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, esperado %d — %s", w.Code, status, w.Body.String())
	}
	r := leer(t, w)
	if r.Success || r.Error == nil || r.Error.Code != code {
		t.Fatalf("code esperado %s, cuerpo: %s", code, w.Body.String())
	}
	return r
}

// camposConError devuelve los nombres de campo de details (VALIDACION_FALLIDA).
func camposConError(t *testing.T, r respuesta) []string {
	t.Helper()
	lista, _ := r.Error.Details.([]any)
	var campos []string
	for _, d := range lista {
		if m, ok := d.(map[string]any); ok {
			campos = append(campos, m["campo"].(string))
		}
	}
	return campos
}

func multipartForm(t *testing.T, campos map[string]string, nombreArchivo string, archivo []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range campos {
		_ = mw.WriteField(k, v)
	}
	if nombreArchivo != "" {
		fw, err := mw.CreateFormFile("foto", nombreArchivo)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(archivo)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

// ---------- dobles de repositorio ----------

type usuariosEnMemoria struct {
	porCorreo map[string]*models.Usuario
	falla     error
}

func (r *usuariosEnMemoria) Crear(_ context.Context, u *models.Usuario) error {
	if r.falla != nil {
		return r.falla
	}
	if _, ok := r.porCorreo[u.Correo]; ok {
		return repositories.ErrCorreoDuplicado
	}
	u.ID = int64(len(r.porCorreo) + 1)
	u.FechaRegistro, u.FechaActualizacion = time.Now(), time.Now()
	copia := *u
	r.porCorreo[u.Correo] = &copia
	return nil
}

func (r *usuariosEnMemoria) BuscarPorCorreo(_ context.Context, correo string) (*models.Usuario, error) {
	if r.falla != nil {
		return nil, r.falla
	}
	if u, ok := r.porCorreo[correo]; ok {
		return u, nil
	}
	return nil, repositories.ErrNoEncontrado
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

type estadosEnMemoria struct {
	estados map[int64]dominio.Estado
	falla   error
}

func (r *estadosEnMemoria) Cambiar(_ context.Context, id, idUsuario int64, destino dominio.Estado,
	validar func(dominio.Estado) error) (*models.CambioEstado, error) {
	if r.falla != nil {
		return nil, r.falla
	}
	actual, ok := r.estados[id]
	if !ok {
		return nil, repositories.ErrNoEncontrado
	}
	if err := validar(actual); err != nil {
		return nil, err
	}
	r.estados[id] = destino
	return &models.CambioEstado{IDReporte: id, EstadoAnterior: actual, EstadoNuevo: destino,
		CambiadoPor: idUsuario, Fecha: time.Now(), FechaActualizacion: time.Now()}, nil
}
