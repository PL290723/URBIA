package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// ConsultaReporteService entrega el listado y el detalle de reportes (HU 06.4)
// recortando los datos según quién consulta (dominio.Vista):
//   - publica (reporte de otro ciudadano): estado, categoría, descripción y
//     ubicación aproximada (~110 m). Sin autor, sin foto, sin datos de la IA.
//   - autor: además ubicación exacta, foto, sugerencia de la IA e historial.
//   - administrador: todo, incluidos autor, prioridad y detalle de la detección.
type ConsultaReporteService struct {
	repo *repositories.ConsultaReporteRepository
	log  *slog.Logger
}

func NewConsultaReporteService(repo *repositories.ConsultaReporteRepository, log *slog.Logger) *ConsultaReporteService {
	return &ConsultaReporteService{repo: repo, log: log}
}

// Listar devuelve una página de reportes con los filtros pedidos.
func (s *ConsultaReporteService) Listar(ctx context.Context, sesion auth.Sesion, f dominio.FiltrosReporte) (*models.ListadoReportes, error) {
	if f.BBox != nil && sesion.Rol != dominio.RolAdministrador {
		// El rectángulo se ajusta a la cuadrícula pública para no delatar ubicaciones exactas.
		ajustado := dominio.AjustarBBoxPublica(*f.BBox)
		f.BBox = &ajustado
	}

	filas, total, err := s.repo.Listar(ctx, f, sesion.UsuarioID)
	if err != nil {
		return nil, err
	}

	elementos := make([]models.ReporteResumen, len(filas))
	for i := range filas {
		vista := dominio.VistaDeReporte(sesion.Rol, sesion.UsuarioID, filas[i].IDUsuario)
		elementos[i] = resumenDe(&filas[i], vista, sesion.UsuarioID)
	}
	totalPaginas := int((total + int64(f.PorPagina) - 1) / int64(f.PorPagina))

	return &models.ListadoReportes{
		Elementos: elementos,
		Paginacion: models.Paginacion{
			Pagina: f.Pagina, PorPagina: f.PorPagina, Total: total, TotalPaginas: totalPaginas,
		},
	}, nil
}

// Detalle devuelve un reporte. Cualquier usuario autenticado puede verlo, con
// el nivel de detalle de su vista.
func (s *ConsultaReporteService) Detalle(ctx context.Context, sesion auth.Sesion, id int64) (*models.ReporteDetalle, error) {
	// Primero sin extras para saber quién es el autor; solo si la vista lo
	// permite se leen la detección y el historial.
	fila, err := s.repo.Detalle(ctx, id, false)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, response.NotFound(msgReporteNoEncontrado)
	}
	if err != nil {
		return nil, err
	}

	vista := dominio.VistaDeReporte(sesion.Rol, sesion.UsuarioID, fila.IDUsuario)
	if vista.VePrivado() {
		if fila, err = s.repo.Detalle(ctx, id, true); err != nil {
			return nil, err
		}
	}
	return detalleDe(fila, vista, sesion.UsuarioID), nil
}

// ---------------------------------------------------------------------
// Proyección por vista: funciones puras, probadas sin base de datos.
// ---------------------------------------------------------------------

func ubicacionDe(f *models.FilaReporte, v dominio.Vista) models.UbicacionVista {
	if v.VePrivado() {
		return models.UbicacionVista{Latitud: f.Latitud, Longitud: f.Longitud}
	}
	lat, lon := dominio.UbicacionAproximada(f.Latitud, f.Longitud)
	return models.UbicacionVista{Latitud: lat, Longitud: lon, Aproximada: true}
}

func adminDe(f *models.FilaReporte) *models.DatosAdministrativos {
	return &models.DatosAdministrativos{
		Autor:           models.AutorReporte{IDUsuario: f.IDUsuario, Nombre: f.NombreAutor, Correo: f.CorreoAutor},
		Prioridad:       f.Prioridad,
		OrigenUbicacion: f.OrigenUbicacion,
		PrecisionM:      f.PrecisionM,
		OrigenCategoria: f.OrigenCategoria,
	}
}

func resumenDe(f *models.FilaReporte, v dominio.Vista, idUsuario int64) models.ReporteResumen {
	r := models.ReporteResumen{
		ID:            f.ID,
		Estado:        f.Estado,
		Categoria:     f.Categoria,
		Descripcion:   f.Descripcion,
		Ubicacion:     ubicacionDe(f, v),
		FechaCreacion: f.FechaCreacion,
		EsPropio:      idUsuario > 0 && f.IDUsuario == idUsuario,
	}
	if v.VePrivado() && f.Foto != nil {
		id := f.Foto.ID
		r.IDFoto = &id
	}
	if v.VeAdministrativo() {
		r.Admin = adminDe(f)
	}
	return r
}

func detalleDe(f *models.FilaReporte, v dominio.Vista, idUsuario int64) *models.ReporteDetalle {
	d := &models.ReporteDetalle{
		ID:                 f.ID,
		Estado:             f.Estado,
		Categoria:          f.Categoria,
		Descripcion:        f.Descripcion,
		Ubicacion:          ubicacionDe(f, v),
		FechaCreacion:      f.FechaCreacion,
		FechaActualizacion: f.FechaActualizacion,
		EsPropio:           idUsuario > 0 && f.IDUsuario == idUsuario,
	}
	if !v.VePrivado() {
		return d
	}

	if f.Foto != nil {
		d.Foto = &models.FotoVista{
			IDArchivo:   f.Foto.ID,
			MIME:        f.Foto.MIME,
			Tamano:      f.Foto.Tamano,
			FechaSubida: f.Foto.FechaSubida,
			URL:         fmt.Sprintf("/api/v1/archivos/%d", f.Foto.ID), // misma política de acceso
		}
	}
	if f.Deteccion != nil {
		sug := &models.SugerenciaVista{
			Estado:    dominio.EstadoDeSugerencia(f.Deteccion.Resultado, f.CategoriaSugerida != nil),
			Categoria: f.CategoriaSugerida,
		}
		if f.CategoriaSugerida != nil {
			sug.Confianza = f.Deteccion.Confianza
		}
		d.Sugerencia = sug
	}
	d.Historial = f.Historial

	if v.VeAdministrativo() {
		d.Admin = adminDe(f)
		if det := f.Deteccion; det != nil {
			d.Admin.Deteccion = &models.DeteccionVista{
				IDDeteccion:    det.ID,
				Resultado:      det.Resultado,
				ClaseDetectada: det.ClaseDetectada,
				Confianza:      det.Confianza,
				Modelo:         det.Modelo,
				VersionModelo:  det.VersionModelo,
				CodigoError:    det.CodigoError,
				Fecha:          det.Fecha,
			}
		}
	}
	return d
}
