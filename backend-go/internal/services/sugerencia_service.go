package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
)

// Detector es lo que la sugerencia necesita del cliente gRPC (clients.ClienteDeteccion).
// Con una interfaz se puede probar sin levantar el servicio Python.
type Detector interface {
	Detectar(ctx context.Context, s clients.SolicitudDeteccion) (*clients.Deteccion, error)
}

var errNoConfigurado = errors.New("servicio de inferencia no configurado (INFERENCE_GRPC_ADDR vacía)")

// SugerenciaService pide a la IA una categoría para un reporte YA guardado (HU 06.2).
// Nunca devuelve error: si algo falla, el reporte ya existe y se responde
// con la sugerencia "no_disponible".
type SugerenciaService struct {
	detector    Detector // nil si INFERENCE_GRPC_ADDR no está configurada
	detecciones *repositories.DeteccionRepository
	categorias  *repositories.CategoriaRepository
	log         *slog.Logger
}

func NewSugerenciaService(detector Detector, detecciones *repositories.DeteccionRepository,
	categorias *repositories.CategoriaRepository, log *slog.Logger) *SugerenciaService {
	return &SugerenciaService{detector: detector, detecciones: detecciones, categorias: categorias, log: log}
}

// Sugerir llama a la IA, guarda la detección original y arma la sugerencia.
// NO modifica reporte.id_categoria: la categoría que eligió el ciudadano se respeta siempre.
func (s *SugerenciaService) Sugerir(ctx context.Context, rep *models.Reporte, idSolicitud string) *models.Sugerencia {
	// El reporte ya está confirmado: aunque el cliente cierre la conexión, la
	// detección se termina y se guarda. El límite de tiempo lo pone el cliente gRPC.
	ctx = context.WithoutCancel(ctx)

	var det *clients.Deteccion
	err := errNoConfigurado
	if s.detector != nil {
		det, err = s.detector.Detectar(ctx, clients.SolicitudDeteccion{
			IDSolicitud: idSolicitud,
			IDReporte:   rep.ID,
			IDArchivo:   rep.Foto.ID,
			Ruta:        rep.Foto.Ruta,
			OmitirCajas: true, // para la sugerencia basta con la clase y la confianza
		})
	}
	d := nuevaDeteccion(rep.ID, det, err)
	if err != nil {
		s.log.Warn("inferencia no disponible; el reporte se conserva sin sugerencia",
			"id_reporte", rep.ID, "codigo", *d.CodigoError, "error", err.Error())
	}

	var idDeteccion *int64
	if errGuardar := s.detecciones.Crear(ctx, d); errGuardar != nil {
		// Tampoco esto tumba la respuesta: el reporte ya está guardado.
		s.log.Error("no se pudo guardar la detección", "id_reporte", rep.ID, "error", errGuardar)
	} else {
		idDeteccion = &d.ID
	}

	sug := &models.Sugerencia{
		Modelo:        d.Modelo,
		VersionModelo: d.VersionModelo,
		IDDeteccion:   idDeteccion,
	}
	if d.Resultado == dominio.ResultadoDetectado {
		cat, errCat := s.categorias.BuscarActivaPorNombre(ctx, *d.ClaseDetectada)
		switch {
		case errCat == nil:
			sug.Categoria, sug.Confianza = cat, d.Confianza
		case errors.Is(errCat, repositories.ErrNoEncontrado):
			s.log.Warn("la clase detectada no corresponde a una categoría activa",
				"id_reporte", rep.ID, "clase", *d.ClaseDetectada)
		default:
			s.log.Error("no se pudo leer la categoría sugerida", "id_reporte", rep.ID, "error", errCat)
		}
	}
	sug.Estado = dominio.EstadoDeSugerencia(d.Resultado, sug.Categoria != nil)

	var confirmada, sugerida *int16
	if rep.Categoria != nil {
		confirmada = &rep.Categoria.ID
	}
	if sug.Categoria != nil {
		sugerida = &sug.Categoria.ID
	}
	sug.CoincideConCategoria = dominio.CoincideConCategoria(confirmada, sugerida)

	s.log.Info("sugerencia de categoría", "id_reporte", rep.ID, "estado", sug.Estado,
		"resultado", d.Resultado, "con_categoria_ciudadano", rep.Categoria != nil)
	return sug
}

// nuevaDeteccion convierte la respuesta (o el error) de la IA en la fila de deteccion.
// Es una función pura para poder probar todos los casos sin base de datos.
func nuevaDeteccion(idReporte int64, det *clients.Deteccion, err error) *models.Deteccion {
	d := &models.Deteccion{IDReporte: idReporte}
	if err != nil || det == nil {
		codigo := clients.CodigoDesconocido
		var ei *clients.ErrorInferencia
		switch {
		case errors.Is(err, errNoConfigurado):
			codigo = dominio.CodigoNoConfigurado
		case errors.As(err, &ei) && ei.Codigo != "":
			codigo = ei.Codigo
		}
		d.Resultado, d.CodigoError = dominio.ResultadoError, &codigo
		return d
	}

	modelo, version, umbral, ms := det.Modelo, det.VersionModelo, det.UmbralConfianza, det.Milisegundos
	d.Modelo, d.VersionModelo, d.UmbralConfianza, d.Milisegundos = &modelo, &version, &umbral, &ms
	if det.Resultado == clients.ResultadoDetectado && det.ClaseDetectada != nil && det.Confianza != nil {
		d.Resultado, d.ClaseDetectada, d.Confianza = dominio.ResultadoDetectado, det.ClaseDetectada, det.Confianza
	} else {
		d.Resultado = dominio.ResultadoNoDetectado // clase y confianza quedan NULL
	}
	return d
}
