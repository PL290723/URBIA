package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

const (
	msgSoloAdminPrioridad = "Solo un administrador puede calcular la prioridad"
	msgReporteResuelto    = "El reporte ya está resuelto; no necesita prioridad"
	motivoSinCategoria    = "el reporte no tiene categoría confirmada, así que no se puede comparar con otros"
)

// PrioridadService calcula una prioridad provisional y explicable (HU 07.1).
type PrioridadService struct {
	repo     *repositories.PrioridadRepository
	cfg      dominio.ConfigPrioridad
	cercania dominio.ParametrosCercania // radio y ventana de la HU 06.5 (DUPLICADOS_*)
	log      *slog.Logger
	ahora    func() time.Time
}

func NewPrioridadService(repo *repositories.PrioridadRepository, cfg dominio.ConfigPrioridad,
	cercania dominio.ParametrosCercania, log *slog.Logger) *PrioridadService {
	return &PrioridadService{repo: repo, cfg: cfg, cercania: cercania, log: log, ahora: time.Now}
}

// Calcular evalúa la fórmula con los datos actuales, guarda el resultado con sus
// factores y, si los datos alcanzan, asigna la prioridad al reporte. Llamarlo de
// nuevo es "recalcular": los campos omitidos reutilizan la evaluación anterior.
func (s *PrioridadService) Calcular(ctx context.Context, sesion auth.Sesion, id int64,
	req models.CalcularPrioridadRequest) (*models.CalculoPrioridad, error) {
	if !sesion.Rol.PuedeCalcularPrioridad() { // la ruta ya lo filtra; defensa adicional
		return nil, response.Forbidden(msgSoloAdminPrioridad)
	}

	eval, errs := dominio.ValidarEvaluacion(dominio.EvaluacionAdmin{
		Gravedad:                req.GravedadObservable,
		Afectacion:              req.AfectacionEstimada,
		JustificacionAfectacion: req.JustificacionAfectacion,
	})
	if len(errs) > 0 {
		return nil, response.Validation(errs)
	}

	rep, err := s.leerReporte(ctx, id)
	if err != nil {
		return nil, err
	}
	if !rep.Estado.PermiteCalcularPrioridad() {
		return nil, response.Conflict(msgReporteResuelto)
	}

	anterior, err := s.ultimo(ctx, id)
	if err != nil {
		return nil, err
	}
	eval = combinarEvaluacion(eval, anterior)

	calculo, err := s.evaluar(ctx, rep, eval)
	if err != nil {
		return nil, err
	}
	calculo.CalculadoPor = sesion.UsuarioID

	if err := s.repo.Guardar(ctx, calculo); err != nil {
		if errors.Is(err, repositories.ErrNoEncontrado) {
			return nil, response.NotFound(msgReporteNoEncontrado)
		}
		return nil, err
	}

	s.log.Info("prioridad calculada", "id_reporte", id, "id_usuario", sesion.UsuarioID,
		"puntuacion", calculo.Puntuacion, "nivel", calculo.Nivel, "cobertura", calculo.Cobertura,
		"faltantes", calculo.Faltantes, "asignada", calculo.PrioridadAsignada)
	return calculo, nil
}

// Consultar devuelve el último cálculo guardado y si sigue vigente con los datos
// de hoy. No guarda nada.
func (s *PrioridadService) Consultar(ctx context.Context, sesion auth.Sesion, id int64) (*models.ConsultaPrioridad, error) {
	if !sesion.Rol.PuedeCalcularPrioridad() {
		return nil, response.Forbidden("Solo un administrador puede consultar la prioridad calculada")
	}
	rep, err := s.leerReporte(ctx, id)
	if err != nil {
		return nil, err
	}
	anterior, err := s.ultimo(ctx, id)
	if err != nil {
		return nil, err
	}

	var eval dominio.EvaluacionAdmin
	if anterior != nil {
		eval = anterior.Evaluacion
	}
	actual, err := s.evaluar(ctx, rep, eval)
	if err != nil {
		return nil, err
	}

	return &models.ConsultaPrioridad{
		Reporte: models.PrioridadReporteVista{
			IDReporte: rep.ID, Estado: rep.Estado,
			Prioridad: rep.Prioridad, OrigenPrioridad: rep.OrigenPrioridad,
		},
		Calculo:  anterior,
		Vigencia: compararVigencia(anterior, actual, rep.Estado),
	}, nil
}

func (s *PrioridadService) leerReporte(ctx context.Context, id int64) (*models.ReportePrioridadFila, error) {
	rep, err := s.repo.Reporte(ctx, id)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, response.NotFound(msgReporteNoEncontrado)
	}
	return rep, err
}

func (s *PrioridadService) ultimo(ctx context.Context, id int64) (*models.CalculoPrioridad, error) {
	c, err := s.repo.Ultimo(ctx, id)
	if errors.Is(err, repositories.ErrNoEncontrado) {
		return nil, nil
	}
	return c, err
}

// evaluar junta los datos REALES disponibles y aplica la fórmula (sin guardar).
func (s *PrioridadService) evaluar(ctx context.Context, rep *models.ReportePrioridadFila,
	eval dominio.EvaluacionAdmin) (*models.CalculoPrioridad, error) {
	datos := dominio.DatosPrioridad{
		Evaluacion:     eval,
		RadioM:         s.cercania.RadioM,
		VentanaDias:    s.cercania.VentanaDias,
		DiasAntiguedad: s.ahora().Sub(rep.FechaCreacion).Hours() / 24,
	}
	var idCategoria *int16
	if rep.Categoria == nil {
		datos.MotivoSinRecurrencia = motivoSinCategoria
	} else {
		idCategoria = &rep.Categoria.ID
		n, ids, err := s.repo.Recurrencias(ctx, rep.ID, s.cercania.RadioM, s.cercania.VentanaDias)
		if err != nil {
			return nil, err
		}
		datos.Recurrencias, datos.IDsRecurrentes = &n, ids
	}
	return armarCalculo(rep.ID, datos, idCategoria, s.cfg), nil
}

// armarCalculo aplica la fórmula y prepara la fila a guardar. Función pura.
func armarCalculo(idReporte int64, d dominio.DatosPrioridad, idCategoria *int16,
	cfg dominio.ConfigPrioridad) *models.CalculoPrioridad {
	r := dominio.CalcularPrioridad(d, cfg)
	return &models.CalculoPrioridad{
		IDReporte:          idReporte,
		ResultadoPrioridad: r,
		PrioridadAsignada:  r.Nivel != nil, // con datos insuficientes NO se toca reporte.prioridad
		Evaluacion:         d.Evaluacion,
		IDCategoria:        idCategoria,
		Recurrencias:       d.Recurrencias,
		DiasAntiguedad:     redondear1(d.DiasAntiguedad),
	}
}

// combinarEvaluacion: lo que no manda el administrador se toma del cálculo anterior.
// Son datos que él mismo declaró antes, no valores inventados.
func combinarEvaluacion(nueva dominio.EvaluacionAdmin, anterior *models.CalculoPrioridad) dominio.EvaluacionAdmin {
	if anterior == nil {
		return nueva
	}
	if nueva.Gravedad == nil {
		nueva.Gravedad = anterior.Evaluacion.Gravedad
	}
	if nueva.Afectacion == nil {
		nueva.Afectacion = anterior.Evaluacion.Afectacion
		nueva.JustificacionAfectacion = anterior.Evaluacion.JustificacionAfectacion
	}
	return nueva
}

// compararVigencia explica qué cambió desde el último cálculo. Función pura.
func compararVigencia(anterior, actual *models.CalculoPrioridad, estado dominio.Estado) models.VigenciaPrioridad {
	v := models.VigenciaPrioridad{
		Motivos:          []string{},
		PuntuacionActual: actual.Puntuacion,
		NivelActual:      actual.Nivel,
	}
	if anterior == nil {
		v.Desactualizado = true
		v.Motivos = append(v.Motivos, "El reporte aún no tiene una prioridad calculada")
		return v
	}
	if !estado.PermiteCalcularPrioridad() {
		v.Motivos = append(v.Motivos, "El reporte ya está resuelto; no se recalcula")
		return v
	}
	if anterior.VersionFormula != actual.VersionFormula {
		v.Motivos = append(v.Motivos, fmt.Sprintf("Cambió la versión de la fórmula (%s → %s)",
			anterior.VersionFormula, actual.VersionFormula))
	}
	if !mismoInt16(anterior.IDCategoria, actual.IDCategoria) {
		v.Motivos = append(v.Motivos, "Cambió la categoría del reporte")
	}
	if !mismoInt(anterior.Recurrencias, actual.Recurrencias) {
		v.Motivos = append(v.Motivos, fmt.Sprintf("Cambió la recurrencia verificada (%s → %s reportes)",
			textoInt(anterior.Recurrencias), textoInt(actual.Recurrencias)))
	}
	if textoNivel(anterior.Nivel) != textoNivel(actual.Nivel) {
		v.Motivos = append(v.Motivos, fmt.Sprintf("Con los datos de hoy el nivel pasaría de %s a %s",
			textoNivel(anterior.Nivel), textoNivel(actual.Nivel)))
	}
	v.Desactualizado = len(v.Motivos) > 0
	return v
}

func mismoInt16(a, b *int16) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func mismoInt(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func textoInt(v *int) string {
	if v == nil {
		return "no verificable"
	}
	return fmt.Sprint(*v)
}

func textoNivel(n *dominio.NivelPrioridad) string {
	if n == nil {
		return "sin nivel"
	}
	return string(*n)
}

func redondear1(v float64) float64 {
	return math.Round(math.Max(0, v)*10) / 10
}
