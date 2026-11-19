// Herramientas del administrador dentro del detalle de un reporte:
// cambio de estado, prioridad explicable y posibles duplicados cercanos.
import { useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { Check } from 'lucide-react'
import { Circle } from 'react-leaflet'
import { useCalcularPrioridad, useCambiarEstado, useCercanos, usePrioridad } from '../../api/consultas'
import { ApiError } from '../../api/client'
import type { CalculoPrioridad, ReporteDetalle } from '../../api/tipos'
import { MapaReportes } from '../../components/mapa'
import { Aviso, Cargando, InsigniaEstado, InsigniaPrioridad, MensajeError, Spinner } from '../../components/ui'
import { ESTADO_INFO, FACTOR_PRIORIDAD, nombreCategoria, siguienteEstado } from '../../lib/catalogos'
import { fechaHora, metros } from '../../lib/formato'

function CambiarEstado({ reporte }: { reporte: ReporteDetalle }) {
  const cambiar = useCambiarEstado(reporte.id_reporte)
  const siguiente = siguienteEstado(reporte.estado)
  const [confirmando, setConfirmando] = useState(false)

  return (
    <section className="hoja p-5">
      <h3 className="seccion-titulo">Avanzar el estado</h3>
      {siguiente ? (
        <>
          <p className="mt-1 text-sm text-gris">
            Está en <strong className="text-asfalto">{ESTADO_INFO[reporte.estado].etiqueta.toLowerCase()}</strong>. El
            siguiente paso es <strong className="text-asfalto">{ESTADO_INFO[siguiente].etiqueta.toLowerCase()}</strong>;
            un cambio no se puede deshacer y queda en el historial con tu nombre.
          </p>
          {confirmando ? (
            <div className="mt-4 flex flex-wrap gap-2">
              <button
                type="button"
                className="btn-primario"
                disabled={cambiar.isPending}
                onClick={() => cambiar.mutate(siguiente, { onSuccess: () => setConfirmando(false) })}
              >
                {cambiar.isPending ? <Spinner /> : <Check size={18} aria-hidden />} Confirmar cambio
              </button>
              <button type="button" className="btn-secundario" onClick={() => setConfirmando(false)}>
                Cancelar
              </button>
            </div>
          ) : (
            <button type="button" className="btn-primario mt-4 w-full sm:w-auto" onClick={() => setConfirmando(true)}>
              Pasar a {ESTADO_INFO[siguiente].etiqueta.toLowerCase()}
            </button>
          )}
        </>
      ) : (
        <div className="mt-3">
          <Aviso tipo="exito">Reporte resuelto. Es el último estado del flujo.</Aviso>
        </div>
      )}
      {cambiar.isError && (
        <div className="mt-3">
          <MensajeError error={cambiar.error} />
        </div>
      )}
    </section>
  )
}

function DesgloseCalculo({ c }: { c: CalculoPrioridad }) {
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <span className="titulo text-4xl text-asfalto tabular-nums">{c.puntuacion.toFixed(1)}</span>
        <span className="text-sm text-gris">puntos de 100</span>
        {!c.nivel && <span className="text-sm text-amber-700">sin nivel asignado</span>}
      </div>
      <p className="text-sm text-gris">{c.resumen}</p>
      <ul className="space-y-3">
        {c.factores.map((f) => (
          <li key={f.factor}>
            <div className="flex items-baseline justify-between gap-2 text-sm">
              <span className="font-medium">
                {FACTOR_PRIORIDAD[f.factor] ?? f.factor}
                <span className="ml-1.5 font-normal text-gris-claro">peso {(f.peso * 100).toFixed(0)} %</span>
              </span>
              <span className={`tabular-nums ${f.disponible ? 'font-semibold' : 'text-gris-claro'}`}>
                {f.disponible && f.aporte != null ? `+${f.aporte.toFixed(1)}` : 'Sin dato'}
              </span>
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-concreto">
              <div className="h-full rounded-full bg-urbia-700" style={{ width: `${(f.valor ?? 0) * 100}%` }} />
            </div>
            <p className="mt-1 text-xs text-gris">{f.explicacion}</p>
          </li>
        ))}
      </ul>
      {!c.datos_suficientes && (
        <Aviso tipo="alerta">
          Solo hay datos para el {(c.cobertura * 100).toFixed(0)} % de la fórmula (mínimo{' '}
          {(c.cobertura_minima * 100).toFixed(0)} %). No se asignó nivel.
        </Aviso>
      )}
      <p className="text-xs text-gris">Calculado el {fechaHora(c.fecha)} con la fórmula {c.version_formula}</p>
    </div>
  )
}

function Escala({ valor, onCambio, nombre }: { valor: number | ''; onCambio: (v: number | '') => void; nombre: string }) {
  return (
    <div className="flex gap-1" role="radiogroup" aria-label={nombre}>
      {[1, 2, 3, 4, 5].map((n) => (
        <button
          key={n}
          type="button"
          role="radio"
          aria-checked={valor === n}
          onClick={() => onCambio(valor === n ? '' : n)}
          className={`titulo size-11 rounded-[10px] text-lg ring-1 ${
            valor === n ? 'bg-urbia-800 text-white ring-urbia-800' : 'bg-white text-asfalto ring-linea hover:ring-urbia-300'
          }`}
        >
          {n}
        </button>
      ))}
    </div>
  )
}

function Prioridad({ reporte }: { reporte: ReporteDetalle }) {
  const consulta = usePrioridad(reporte.id_reporte)
  const calcular = useCalcularPrioridad(reporte.id_reporte)
  const [gravedad, setGravedad] = useState<number | ''>('')
  const [afectacion, setAfectacion] = useState<number | ''>('')
  const [justificacion, setJustificacion] = useState('')
  const resuelto = reporte.estado === 'resuelto'
  const err = calcular.error instanceof ApiError ? calcular.error : null

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    const cuerpo: Record<string, unknown> = {}
    if (gravedad !== '') cuerpo.gravedad_observable = gravedad
    if (afectacion !== '') {
      cuerpo.afectacion_estimada = afectacion
      cuerpo.justificacion_afectacion = justificacion.trim()
    }
    calcular.mutate(Object.keys(cuerpo).length ? cuerpo : null)
  }

  return (
    <section className="hoja p-5">
      <h3 className="seccion-titulo mb-1">Prioridad</h3>
      {consulta.isPending ? (
        <Cargando />
      ) : consulta.isError ? (
        <MensajeError error={consulta.error} reintentar={consulta.refetch} />
      ) : (
        <>
          <p className="mb-4 flex flex-wrap items-center gap-2 text-sm text-gris">
            Asignada: <InsigniaPrioridad nivel={consulta.data.reporte.prioridad} />
            {consulta.data.reporte.origen_prioridad !== 'calculada' && <span>(valor inicial, sin calcular)</span>}
          </p>
          {consulta.data.vigencia.desactualizado && (
            <div className="mb-3">
              <Aviso tipo="alerta">
                El cálculo está desactualizado
                {consulta.data.vigencia.motivos?.length ? `: ${consulta.data.vigencia.motivos.join('; ')}` : '.'}
              </Aviso>
            </div>
          )}
          {calcular.data ? (
            <DesgloseCalculo c={calcular.data} />
          ) : consulta.data.calculo ? (
            <DesgloseCalculo c={consulta.data.calculo} />
          ) : (
            <p className="text-sm text-gris">Todavía no se calcula. Evalúa la gravedad y la afectación abajo.</p>
          )}
        </>
      )}

      {!resuelto && (
        <form onSubmit={enviar} className="mt-5 space-y-4 border-t border-linea pt-5">
          <p className="text-sm text-gris">Lo que no marques se toma de tu evaluación anterior.</p>
          <div>
            <span className="etiqueta">Gravedad observable (1–5)</span>
            <Escala valor={gravedad} onCambio={setGravedad} nombre="Gravedad" />
          </div>
          <div>
            <span className="etiqueta">Afectación estimada (1–5)</span>
            <Escala valor={afectacion} onCambio={setAfectacion} nombre="Afectación" />
          </div>
          {afectacion !== '' && (
            <label className="block">
              <span className="etiqueta">Justificación de la afectación</span>
              <textarea
                className="campo min-h-20"
                minLength={10}
                maxLength={500}
                value={justificacion}
                onChange={(e) => setJustificacion(e.target.value)}
                placeholder="Ej. Paso diario de alumnos hacia la escuela"
              />
              {err?.errorDe('justificacion_afectacion') && (
                <span className="text-sm text-red-700">{err.errorDe('justificacion_afectacion')}</span>
              )}
            </label>
          )}
          {calcular.isError && err?.code !== 'VALIDACION_FALLIDA' && <MensajeError error={calcular.error} />}
          <button
            type="submit"
            className="btn-primario w-full"
            disabled={calcular.isPending || (afectacion !== '' && justificacion.trim().length < 10)}
          >
            {calcular.isPending && <Spinner />} Calcular prioridad
          </button>
        </form>
      )}
    </section>
  )
}

function Cercanos({ reporte }: { reporte: ReporteDetalle }) {
  const [radio, setRadio] = useState(50)
  const [ventana, setVentana] = useState(30)
  const { data, isPending, isError, error, refetch, isFetching } = useCercanos(reporte.id_reporte, {
    radio_m: radio,
    ventana_dias: ventana,
    limite: 20,
  })

  const base = {
    id_reporte: reporte.id_reporte,
    estado: reporte.estado,
    categoria: reporte.categoria,
    descripcion: reporte.descripcion,
    fecha_creacion: reporte.fecha_creacion,
    ubicacion: reporte.ubicacion,
  }

  return (
    <section className="hoja p-5 lg:col-span-2">
      <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h3 className="seccion-titulo">Reportes parecidos cerca</h3>
          <p className="text-sm text-gris">Posibles duplicados. Es solo una sugerencia; nada se une automáticamente.</p>
        </div>
        <div className="flex gap-2 text-sm">
          <label>
            <span className="etiqueta">Distancia</span>
            <select className="campo" value={radio} onChange={(e) => setRadio(Number(e.target.value))}>
              {[25, 50, 100, 250, 500, 1000].map((m) => (
                <option key={m} value={m}>
                  {metros(m)}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span className="etiqueta">Periodo</span>
            <select className="campo" value={ventana} onChange={(e) => setVentana(Number(e.target.value))}>
              {[7, 30, 90, 365].map((d) => (
                <option key={d} value={d}>
                  {d} días
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      {isPending ? (
        <Cargando />
      ) : isError ? (
        <MensajeError error={error} reintentar={refetch} />
      ) : (
        <div className={`grid gap-4 md:grid-cols-2 ${isFetching ? 'opacity-60' : ''}`}>
          <div className="overflow-hidden rounded-xl border border-linea">
            <MapaReportes
              reportes={[base, ...data.coincidencias]}
              resaltado={reporte.id_reporte}
              centro={[reporte.ubicacion.latitud, reporte.ubicacion.longitud]}
              className="h-72 w-full"
              extra={
                <Circle
                  center={[reporte.ubicacion.latitud, reporte.ubicacion.longitud]}
                  radius={radio}
                  pathOptions={{ color: '#144845', weight: 1, dashArray: '4 4', fillOpacity: 0.05 }}
                />
              }
            />
          </div>
          {data.coincidencias.length === 0 ? (
            <p className="self-center text-center text-sm text-gris">
              No hay reportes a menos de {metros(radio)} en {ventana} días.
            </p>
          ) : (
            <ul className="max-h-72 space-y-2 overflow-y-auto">
              {data.coincidencias.map((c) => (
                <li key={c.id_reporte}>
                  <Link to={`/reportes/${c.id_reporte}`} className="flex gap-3 rounded-xl border border-linea p-3 hover:border-urbia-300">
                    <span
                      className={`titulo flex size-12 shrink-0 flex-col items-center justify-center rounded-[10px] text-lg leading-none ${
                        c.nivel === 'alta' ? 'bg-red-50 text-red-800' : c.nivel === 'media' ? 'bg-orange-50 text-orange-800' : 'bg-concreto text-gris'
                      }`}
                      title={`Coincidencia ${c.nivel}: ${Math.round(c.puntuacion * 100)} %`}
                    >
                      {Math.round(c.puntuacion * 100)}
                      <span className="font-sans text-[10px] font-medium">similar</span>
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center justify-between gap-2">
                        <span className="font-semibold">
                          {nombreCategoria(c.categoria)} <span className="font-normal text-gris">folio {c.id_reporte}</span>
                        </span>
                        <InsigniaEstado estado={c.estado} />
                      </span>
                      <span className="mt-0.5 block line-clamp-1 text-sm text-gris">{c.descripcion}</span>
                      <span className="mt-1 block text-xs text-gris">
                        A {metros(c.distancia_m)}, {Math.abs(Math.round(c.dias_diferencia))} días {c.dias_diferencia < 0 ? 'antes' : 'después'}
                        {c.mismo_autor && ', misma persona'}
                      </span>
                    </span>
                  </Link>
                </li>
              ))}
              {data.total > data.coincidencias.length && (
                <li className="text-center text-xs text-gris">
                  Hay {data.total - data.coincidencias.length} más; acota la distancia o el periodo.
                </li>
              )}
            </ul>
          )}
        </div>
      )}
    </section>
  )
}

export default function HerramientasReporte({ reporte }: { reporte: ReporteDetalle }) {
  return (
    <div>
      <h2 className="titulo mb-4 text-2xl">Atención del reporte</h2>
      <div className="grid items-start gap-4 lg:grid-cols-2">
        <CambiarEstado reporte={reporte} />
        <Prioridad reporte={reporte} />
        <Cercanos reporte={reporte} />
      </div>
    </div>
  )
}
