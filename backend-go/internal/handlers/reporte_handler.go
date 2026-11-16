package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

// memoriaMultipart: lo que excede se escribe a un temporal que net/http borra al terminar.
const memoriaMultipart = 1 << 20

type ReporteHandler struct{ svc *services.ReporteService }

func NewReporteHandler(svc *services.ReporteService) *ReporteHandler {
	return &ReporteHandler{svc: svc}
}

// Crear: POST /api/v1/reportes (multipart/form-data)
//
//	foto              archivo  obligatorio  JPEG, PNG o WebP, máx. 10 MB
//	descripcion       texto    obligatorio  10 a 1000 caracteres
//	latitud, longitud decimal  obligatorio  WGS 84
//	origen_ubicacion  texto    obligatorio  "gps" | "manual"
//	precision_m       decimal  si es gps    0 < p ≤ 100
//	id_categoria      entero   opcional
func (h *ReporteHandler) Crear(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}

	limite := dominio.TamanoMaximoArchivo + margenMultipart
	if c.Request.ContentLength > limite {
		response.HandleError(c, response.PayloadTooLarge("La imagen supera el tamaño máximo de 10 MB"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limite)

	if err := c.Request.ParseMultipartForm(memoriaMultipart); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			response.HandleError(c, response.PayloadTooLarge("La imagen supera el tamaño máximo de 10 MB"))
			return
		}
		_ = c.Error(err)
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest,
			"Envía el reporte como multipart/form-data")
		return
	}

	nuevo, errs := dominio.ParseNuevoReporte(dominio.CamposReporte{
		Descripcion:     c.PostForm("descripcion"),
		IDCategoria:     c.PostForm("id_categoria"),
		Latitud:         c.PostForm("latitud"),
		Longitud:        c.PostForm("longitud"),
		OrigenUbicacion: c.PostForm("origen_ubicacion"),
		PrecisionM:      c.PostForm("precision_m"),
	})

	fh, err := c.FormFile("foto")
	switch {
	case err != nil:
		errs = append(errs, dominio.ErrorCampo{Campo: "foto", Regla: "requerido",
			Mensaje: "Adjunta una fotografía del problema en el campo 'foto'"})
	case !nombreArchivoValido(fh.Filename):
		errs = append(errs, dominio.ErrorCampo{Campo: "foto", Regla: "nombre_invalido",
			Mensaje: "El nombre del archivo no es válido"})
	}
	if len(errs) > 0 {
		response.ValidationError(c, errs)
		return
	}

	f, err := fh.Open()
	if err != nil {
		response.HandleError(c, err)
		return
	}
	defer f.Close()

	rep, err := h.svc.Crear(c.Request.Context(), sesion, nuevo, f)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.Header("Location", fmt.Sprintf("/api/v1/reportes/%d", rep.ID))
	response.Created(c, rep)
}
