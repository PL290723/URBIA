import { lazy, Suspense } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { ArrowLeft, ScanSearch } from 'lucide-react'
import { useReporte } from '../api/consultas'
import type { HistorialEstado } from '../api/tipos'
import { useEsAdmin } from '../auth/useSesion'
import { AvanceEstado } from '../components/AvanceEstado'
import { FotoProtegida } from '../components/FotoProtegida'
import { MapaPunto } from '../components/mapa'
import { Aviso, Cargando, Dato, IconoCategoria, MensajeError } from '../components/ui'
import { ESTADO_INFO, ORIGEN_CATEGORIA, nombreCategoria } from '../lib/catalogos'
import { fechaHora, haceCuanto, porcentaje } from '../lib/formato'

const HerramientasAdmin = lazy(() => import('./admin/HerramientasReporte'))

function Historial({ eventos, esAdmin }: { eventos: HistorialEstado[]; esAdmin: boolean }) {
  return (
    <ol className="space-y-4">
      {eventos.map((e, i) => (
        <li key={e.id_historial_estado} className="relative flex gap-3">
          {i < eventos.length - 1 && <span className="absolute top-5 bottom-[-1rem] left-[5px] w-0.5 bg-linea" aria-hidden />}
          <span className="relative mt-1.5 size-3 shrink-0 rounded-full" style={{ background: ESTADO_INFO[e.estado_nuevo].color }} aria-hidden />
          <div>
            <p className="font-medium">
              {e.tipo === 'creacion' ? 'Reporte recibido' : `Pasó a ${ESTADO_INFO[e.estado_nuevo].etiqueta.toLowerCase()}`}
            </p>
            <p className="text-sm text-gris">
              {fechaHora(e.fecha)}
              {e.actor && e.tipo !== 'creacion' && (
                <>, {esAdmin && e.actor.nombre ? e.actor.nombre : 'por la administración'}</>
              )}
            </p>
          </div>
        </li>
      ))}
    </ol>
  )
}

export function DetalleReporte() {
  const id = Number(useParams().id)
  const navegar = useNavigate()
  const esAdmin = useEsAdmin()
  const { data: r, isPending, isError, error, refetch } = useReporte(id)

  if (!Number.isInteger(id) || id <= 0) return <MensajeError error={new Error('El folio del reporte no es válido')} />
  if (isPending) return <Cargando />
  if (isError) return <MensajeError error={error} reintentar={refetch} />

  const puedeCambiarCategoria = r.es_propio && !esAdmin && r.estado === 'pendiente'
  const adm = r.administrativo

  return (
    <div className="mx-auto max-w-5xl">
      <button type="button" onClick={() => navegar(-1)} className="btn-texto mb-4 text-sm">
        <ArrowLeft size={16} aria-hidden /> Regresar
      </button>

      <header className="mb-6 flex flex-wrap items-start gap-4">
        <IconoCategoria categoria={r.categoria} tamano="lg" />
        <div className="min-w-0 flex-1">
          <h1 className="titulo text-3xl">{nombreCategoria(r.categoria)}</h1>
          <p className="mt-1 text-sm text-gris">
            Folio {r.id_reporte}. Recibido el {fechaHora(r.fecha_creacion)}, última actualización {haceCuanto(r.fecha_actualizacion)}.
          </p>
        </div>
      </header>

      <div className="hoja mb-6 px-4 py-5 sm:px-8">
        <AvanceEstado estado={r.estado} />
      </div>

      <div className="grid gap-6 lg:grid-cols-[3fr_2fr]">
        <div className="space-y-6">
          {r.foto && (
            <FotoProtegida idArchivo={r.foto.id_archivo} alt={r.descripcion} className="max-h-[30rem] w-full rounded-2xl border border-linea bg-white" />
          )}

          <section>
            <h2 className="seccion-titulo mb-2">Descripción</h2>
            <p className="max-w-[65ch] whitespace-pre-line">{r.descripcion}</p>
          </section>

          {r.sugerencia && (
            <section className="flex gap-3 rounded-2xl bg-urbia-50 p-4">
              <ScanSearch size={20} className="mt-0.5 shrink-0 text-urbia-700" aria-hidden />
              <div className="text-sm">
                <p className="font-semibold text-urbia-900">Análisis de la foto</p>
                <p className="mt-0.5 text-urbia-900">
                  {r.sugerencia.estado === 'sugerida'
                    ? `Lo identificó como ${nombreCategoria(r.sugerencia.categoria).toLowerCase()}, con ${porcentaje(r.sugerencia.confianza)} de certeza.`
                    : r.sugerencia.estado === 'sin_deteccion'
                      ? 'No reconoció un problema en la foto.'
                      : 'No estaba disponible cuando se envió el reporte.'}
                </p>
                {puedeCambiarCategoria && (
                  <Link to={`/reportes/${r.id_reporte}/categoria`} className="btn-texto mt-2">
                    {r.categoria ? 'Cambiar el tipo de problema' : 'Elegir el tipo de problema'}
                  </Link>
                )}
              </div>
            </section>
          )}
        </div>

        <div className="space-y-6">
          <section>
            <div className="overflow-hidden rounded-2xl border border-linea">
              <MapaPunto
                lat={r.ubicacion.latitud}
                lon={r.ubicacion.longitud}
                aproximada={r.ubicacion.aproximada}
                estado={r.estado}
                categoria={r.categoria}
              />
            </div>
            <p className="mt-2 text-sm text-gris">
              {r.ubicacion.aproximada
                ? 'Se muestra una zona de unos 110 m para proteger a quien reportó.'
                : `${r.ubicacion.latitud.toFixed(6)}, ${r.ubicacion.longitud.toFixed(6)}`}
            </p>
          </section>

          {r.historial && r.historial.length > 0 && (
            <section>
              <h2 className="seccion-titulo mb-3">Seguimiento</h2>
              <Historial eventos={r.historial} esAdmin={esAdmin} />
            </section>
          )}

          {!r.es_propio && !esAdmin && <Aviso>Este reporte es de otra persona, por eso solo ves los datos generales.</Aviso>}

          {adm && (
            <section className="hoja p-4">
              <h2 className="seccion-titulo mb-3">Datos internos</h2>
              <dl className="grid grid-cols-2 gap-4 text-sm">
                <div className="col-span-2">
                  <Dato etiqueta="Quién reportó">
                    {adm.autor.nombre}
                    <span className="block truncate font-normal text-gris">{adm.autor.correo}</span>
                  </Dato>
                </div>
                <Dato etiqueta="Ubicación">
                  {adm.origen_ubicacion === 'gps' ? 'GPS' : 'Marcada a mano'}
                  {adm.precision_m != null && `, ±${Math.round(adm.precision_m)} m`}
                </Dato>
                <Dato etiqueta="Tipo de problema">{adm.origen_categoria ? ORIGEN_CATEGORIA[adm.origen_categoria] : 'Sin confirmar'}</Dato>
                {adm.deteccion && (
                  <div className="col-span-2">
                    <Dato etiqueta="Detección del modelo">
                      {adm.deteccion.resultado === 'detectado'
                        ? `${adm.deteccion.clase_detectada ?? 'Detectado'}, ${porcentaje(adm.deteccion.confianza)}`
                        : adm.deteccion.resultado === 'no_detectado'
                          ? 'Sin detección'
                          : `Error ${adm.deteccion.codigo_error ?? ''}`}
                      <span className="block font-normal text-gris">
                        {adm.deteccion.modelo} {adm.deteccion.version_modelo}
                      </span>
                    </Dato>
                  </div>
                )}
              </dl>
            </section>
          )}
        </div>
      </div>

      {esAdmin && (
        <Suspense fallback={<Cargando texto="Cargando herramientas" />}>
          <div className="mt-10 border-t border-linea pt-8">
            <HerramientasAdmin reporte={r} />
          </div>
        </Suspense>
      )}
    </div>
  )
}
