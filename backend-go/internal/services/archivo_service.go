package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/imagenes"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
	"github.com/PL290723/URBIA/backend-go/internal/storage"
)

const (
	msgReporteNoEncontrado = "Reporte no encontrado"
	msgArchivoNoEncontrado = "Archivo no encontrado"
)

type ArchivoService struct {
	repo     *repositories.ArchivoRepository
	almacen  *storage.Local
	saneador *imagenes.Saneador
	log      *slog.Logger
}

func NewArchivoService(repo *repositories.ArchivoRepository, almacen *storage.Local,
	saneador *imagenes.Saneador, log *slog.Logger) *ArchivoService {
	return &ArchivoService{repo: repo, almacen: almacen, saneador: saneador, log: log}
}

// NuevoArchivo: MIME debe venir detectado del contenido, no del header ni de la extensión.
type NuevoArchivo struct {
	IDReporte int64
	MIME      string
	Contenido io.Reader
}

// Guardar sube la foto original de un reporte. Solo el autor del reporte o un administrador.
// Guardar valida, sanea y guarda la foto original de un reporte.
// Solo el autor del reporte o un administrador.
func (s *ArchivoService) Guardar(ctx context.Context, sesion auth.Sesion, idReporte int64, contenido io.Reader) (*models.Archivo, error) {
	autor, err := s.repo.AutorDeReporte(ctx, idReporte)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, response.NotFound(msgReporteNoEncontrado)
	}
	if err != nil {
		return nil, err
	}
	if !dominio.PuedeAccederArchivo(sesion.Rol, sesion.UsuarioID, autor) {
		return nil, response.NotFound(msgReporteNoEncontrado) // no revela que el reporte existe
	}

	// Se valida después del permiso: no se procesa nada de quien no puede subir.
	img, err := s.saneador.Sanear(ctx, contenido)
	if err != nil {
		s.log.Info("imagen rechazada", "id_reporte", idReporte, "id_usuario", sesion.UsuarioID, "motivo", err.Error())
		return nil, s.errorDeValidacion(err)
	}
	return s.guardar(ctx, NuevoArchivo{
		IDReporte: idReporte,
		MIME:      img.MIME,
		Contenido: bytes.NewReader(img.Contenido),
	}, dominio.VarianteOriginal)
}

func (s *ArchivoService) errorDeValidacion(err error) error {
	lim := s.saneador.Limites()
	switch {
	case errors.Is(err, imagenes.ErrVacio):
		return response.BadRequest("El archivo está vacío")
	case errors.Is(err, imagenes.ErrDemasiadoGrande):
		return response.PayloadTooLarge("La imagen supera el tamaño máximo de 10 MB")
	case errors.Is(err, imagenes.ErrFormatoNoPermitido):
		return response.UnsupportedMediaType("Formato no permitido: usa JPEG, PNG o WebP")
	case errors.Is(err, imagenes.ErrDimensiones):
		return response.UnprocessableImage(fmt.Sprintf(
			"La imagen debe medir entre %d y %d px por lado y como máximo %d MP",
			lim.MinLado, lim.MaxLado, lim.MaxPixeles/1_000_000))
	case errors.Is(err, imagenes.ErrImagenDanada):
		return response.UnprocessableImage("La imagen está dañada o no se puede procesar")
	default:
		return err // p. ej. el cliente canceló la petición
	}
}

// GuardarProcesada es de uso interno (resultado del modelo, módulo 05): no la expone ninguna ruta.
func (s *ArchivoService) GuardarProcesada(ctx context.Context, n NuevoArchivo) (*models.Archivo, error) {
	return s.guardar(ctx, n, dominio.VarianteProcesada)
}

func (s *ArchivoService) guardar(ctx context.Context, n NuevoArchivo, v dominio.VarianteArchivo) (*models.Archivo, error) {
	ext, ok := dominio.ExtensionPorMIME(n.MIME)
	if !ok {
		return nil, response.BadRequest("Formato no permitido: usa JPEG, PNG o WebP")
	}

	g, err := s.almacen.Guardar(ctx, v, ext, n.Contenido)
	switch {
	case errors.Is(err, storage.ErrVacio):
		return nil, response.BadRequest("El archivo está vacío")
	case errors.Is(err, storage.ErrDemasiadoGrande):
		return nil, response.PayloadTooLarge("La imagen supera el tamaño máximo de 10 MB")
	case err != nil:
		return nil, fmt.Errorf("guardar en disco: %w", err)
	}

	a := &models.Archivo{
		IDReporte: n.IDReporte,
		Tipo:      "imagen",
		Variante:  v,
		MIME:      n.MIME,
		Tamano:    g.Tamano,
		Ruta:      g.Ruta,
		SHA256:    g.SHA256,
	}
	if err := s.repo.Crear(ctx, a); err != nil {
		s.descartar(g.Ruta) // compensación: sin registro en BD no debe quedar el archivo
		if errors.Is(err, repositories.ErrReporteNoExiste) {
			return nil, response.NotFound(msgReporteNoEncontrado)
		}
		return nil, err
	}
	return a, nil
}

// Obtener devuelve los metadatos y el archivo abierto. Quien llama debe cerrar el *os.File.
func (s *ArchivoService) Obtener(ctx context.Context, sesion auth.Sesion, id int64) (*models.Archivo, *os.File, error) {
	a, err := s.autorizado(ctx, sesion, id)
	if err != nil {
		return nil, nil, err
	}
	f, err := s.almacen.Abrir(a.Ruta)
	if errors.Is(err, storage.ErrNoEncontrado) {
		s.log.Error("archivo registrado en BD pero ausente en disco", "id_archivo", a.ID)
		return nil, nil, response.NotFound(msgArchivoNoEncontrado)
	}
	if err != nil {
		return nil, nil, err
	}
	return a, f, nil
}

// Eliminar borra primero el registro (la BD manda) y después el archivo en disco.
func (s *ArchivoService) Eliminar(ctx context.Context, sesion auth.Sesion, id int64) error {
	a, err := s.autorizado(ctx, sesion, id)
	if err != nil {
		return err
	}
	if err := s.repo.Eliminar(ctx, a.ID); err != nil {
		if errors.Is(err, repositories.ErrNoEncontrado) {
			return response.NotFound(msgArchivoNoEncontrado)
		}
		return err
	}
	s.descartar(a.Ruta)
	return nil
}

func (s *ArchivoService) autorizado(ctx context.Context, sesion auth.Sesion, id int64) (*models.Archivo, error) {
	a, err := s.repo.BuscarPorID(ctx, id)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, response.NotFound(msgArchivoNoEncontrado)
	}
	if err != nil {
		return nil, err
	}
	if !dominio.PuedeAccederArchivo(sesion.Rol, sesion.UsuarioID, a.IDAutorReporte) {
		return nil, response.NotFound(msgArchivoNoEncontrado) // mismo 404: no revela que existe
	}
	return a, nil
}

func (s *ArchivoService) descartar(ruta string) {
	if err := s.almacen.Eliminar(ruta); err != nil {
		s.log.Warn("no se pudo borrar el archivo del disco; quedó huérfano", "ruta", ruta, "error", err)
	}
}
