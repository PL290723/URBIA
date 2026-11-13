package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

// margenMultipart cubre los encabezados del multipart además de la imagen.
const margenMultipart = 1 << 20

type ArchivoHandler struct{ svc *services.ArchivoService }

func NewArchivoHandler(svc *services.ArchivoService) *ArchivoHandler {
	return &ArchivoHandler{svc: svc}
}

// Subir: POST /api/v1/reportes/:id/archivos (multipart/form-data, campo "foto")
func (h *ArchivoHandler) Subir(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	idReporte, ok := parseID(c, "id")
	if !ok {
		return
	}

	limite := dominio.TamanoMaximoArchivo + margenMultipart
	if c.Request.ContentLength > limite {
		response.HandleError(c, response.PayloadTooLarge("La imagen supera el tamaño máximo de 10 MB"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limite)

	fh, err := c.FormFile("foto")
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			response.HandleError(c, response.PayloadTooLarge("La imagen supera el tamaño máximo de 10 MB"))
			return
		}
		_ = c.Error(err)
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest,
			"Envía la imagen en el campo 'foto' (multipart/form-data)")
		return
	}

	if !nombreArchivoValido(fh.Filename) {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "Nombre de archivo inválido")
		return
	}

	f, err := fh.Open()
	if err != nil {
		response.HandleError(c, err)
		return
	}
	defer f.Close()

	// El nombre, la extensión y el Content-Type del cliente se ignoran:
	// el formato se decide por el contenido real en imagenes.Saneador.
	a, err := h.svc.Guardar(c.Request.Context(), sesion, idReporte, f)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Created(c, a)
}

// Descargar: GET /api/v1/archivos/:id — nunca hay acceso público directo a la carpeta.
func (h *ArchivoHandler) Descargar(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	a, f, err := h.svc.Obtener(c.Request.Context(), sesion, id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	defer f.Close()

	ext, _ := dominio.ExtensionPorMIME(a.MIME)
	c.Header("Content-Type", a.MIME)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="urbia-%d.%s"`, a.ID, ext))
	http.ServeContent(c.Writer, c.Request, "", a.FechaSubida, f)
}

// Eliminar: DELETE /api/v1/archivos/:id
func (h *ArchivoHandler) Eliminar(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Eliminar(c.Request.Context(), sesion, id); err != nil {
		response.HandleError(c, err)
		return
	}
	response.NoContent(c)
}

// nombreArchivoValido rechaza nombres manipulados. El nombre nunca se usa para
// guardar (se genera uno aleatorio), esto es defensa adicional. Go ya aplica
// filepath.Base al nombre del multipart.
func nombreArchivoValido(nombre string) bool {
	if nombre == "" || nombre == "." || nombre == ".." || len(nombre) > 255 || !utf8.ValidString(nombre) {
		return false
	}
	if strings.ContainsAny(nombre, `/\:`) { // separadores y flujos alternos de NTFS (foto.jpg:oculto)
		return false
	}
	for _, r := range nombre {
		if r < 0x20 || r == 0x7f { // caracteres de control, incluido el byte nulo
			return false
		}
	}
	return true
}
