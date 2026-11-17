package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

type ConsultaReporteHandler struct {
	svc *services.ConsultaReporteService
}

func NewConsultaReporteHandler(svc *services.ConsultaReporteService) *ConsultaReporteHandler {
	return &ConsultaReporteHandler{svc: svc}
}

// Listar: GET /api/v1/reportes
//
//	pagina, por_pagina           paginación (por defecto 1 y 20; máx. 100)
//	estado                       uno o varios separados por coma
//	id_categoria                 entero
//	desde, hasta                 AAAA-MM-DD (hora de Celaya, ambos incluidos)
//	bbox                         min_lon,min_lat,max_lon,max_lat (área visible del mapa)
//	mios                         true = solo mis reportes
func (h *ConsultaReporteHandler) Listar(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	filtros, errs := dominio.ParseFiltrosReporte(dominio.CamposFiltro{
		Pagina:      c.Query("pagina"),
		PorPagina:   c.Query("por_pagina"),
		Estado:      c.Query("estado"),
		IDCategoria: c.Query("id_categoria"),
		Desde:       c.Query("desde"),
		Hasta:       c.Query("hasta"),
		BBox:        c.Query("bbox"),
		Mios:        c.Query("mios"),
	})
	if len(errs) > 0 {
		response.ValidationError(c, errs)
		return
	}

	listado, err := h.svc.Listar(c.Request.Context(), sesion, filtros)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, listado)
}

// Detalle: GET /api/v1/reportes/:id
func (h *ConsultaReporteHandler) Detalle(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	detalle, err := h.svc.Detalle(c.Request.Context(), sesion, id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, detalle)
}

// Historial: GET /api/v1/reportes/:id/historial
// Secuencia de estados en orden cronológico; solo lectura (HU 07.3).
func (h *ConsultaReporteHandler) Historial(c *gin.Context) {
	sesion, ok := sesionRequerida(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	historial, err := h.svc.Historial(c.Request.Context(), sesion, id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, historial)
}
