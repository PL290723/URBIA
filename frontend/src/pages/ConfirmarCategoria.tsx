// Después de crear el reporte: el ciudadano acepta o corrige la categoría que sugirió la IA.
import { useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router'
import { useConfirmarCategoria, useReporte } from '../api/consultas'
import type { Categoria, Reporte } from '../api/tipos'
import { Check } from 'lucide-react'
import { Aviso, Cargando, IconoCategoria, MensajeError, Spinner } from '../components/ui'
import { CATEGORIAS, nombreCategoria } from '../lib/catalogos'
import { porcentaje } from '../lib/formato'

interface Sugerida {
  estado: 'sugerida' | 'sin_deteccion' | 'no_disponible'
  categoria: Categoria | null
  confianza: number | null
}

export function ConfirmarCategoria() {
  const id = Number(useParams().id)
  const navegar = useNavigate()
  const { state } = useLocation() as { state?: { reporte?: Reporte } }
  // Recién creado: la respuesta del POST ya trae la sugerencia; si se recarga, se consulta el detalle.
  const creado = state?.reporte?.id_reporte === id ? state.reporte : undefined
  const detalle = useReporte(creado ? 0 : id)
  const confirmar = useConfirmarCategoria(id)

  const sugerencia: Sugerida | null | undefined = creado ? creado.sugerencia : detalle.data?.sugerencia
  const categoriaActual = creado ? creado.categoria : detalle.data?.categoria
  const estadoReporte = creado ? creado.estado : detalle.data?.estado

  const [eligiendo, setEligiendo] = useState(false)
  const [elegida, setElegida] = useState<number | null>(null)

  if (!creado && detalle.isPending) return <Cargando />
  if (!creado && detalle.isError) return <MensajeError error={detalle.error} reintentar={detalle.refetch} />

  const irAlDetalle = () => navegar(`/reportes/${id}`, { replace: true })
  const guardar = (idCategoria: number) => confirmar.mutate(idCategoria, { onSuccess: irAlDetalle })

  const hayCategoriaSugerida = sugerencia?.estado === 'sugerida' && sugerencia.categoria
  const yaNoPendiente = estadoReporte && estadoReporte !== 'pendiente'

  const sugerida = hayCategoriaSugerida ? sugerencia!.categoria! : null
  const motivoSinSugerencia =
    sugerencia?.estado === 'sin_deteccion'
      ? 'URBIA no reconoció el problema en la foto.'
      : sugerencia?.estado === 'no_disponible'
        ? 'El análisis de fotos no está disponible en este momento.'
        : null

  return (
    <div className="mx-auto max-w-lg">
      <div className="flex items-start gap-4">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-full bg-urbia-800 text-white">
          <Check size={26} strokeWidth={2.5} aria-hidden />
        </span>
        <div>
          <h1 className="titulo text-3xl">Reporte enviado</h1>
          <p className="mt-1 text-gris">Quedó registrado con el folio {id}. Puedes seguir su avance en Mis reportes.</p>
        </div>
      </div>

      {yaNoPendiente ? (
        <div className="mt-8">
          <Aviso tipo="info">La administración ya lo está revisando, así que el tipo de problema ya no se puede cambiar.</Aviso>
        </div>
      ) : (
        <section className="hoja mt-8 p-5">
          {sugerida && !eligiendo ? (
            <>
              <h2 className="seccion-titulo">¿Es correcto el tipo de problema?</h2>
              <p className="mt-1 text-sm text-gris">URBIA analizó tu foto y lo identificó como:</p>
              <div className="mt-4 flex items-center gap-4">
                <IconoCategoria categoria={sugerida} tamano="lg" />
                <div>
                  <p className="titulo text-2xl">{nombreCategoria(sugerida)}</p>
                  <p className="text-sm text-gris">Certeza del análisis: {porcentaje(sugerencia!.confianza)}</p>
                </div>
              </div>
              {categoriaActual && categoriaActual.id_categoria !== sugerida.id_categoria && (
                <p className="mt-3 text-sm text-gris">Tú habías elegido {nombreCategoria(categoriaActual)}.</p>
              )}
              <div className="mt-5 grid gap-2 sm:grid-cols-2">
                <button type="button" className="btn-primario" disabled={confirmar.isPending} onClick={() => guardar(sugerida.id_categoria)}>
                  {confirmar.isPending ? <Spinner /> : <Check size={18} aria-hidden />} Sí, es correcto
                </button>
                <button type="button" className="btn-secundario" onClick={() => setEligiendo(true)}>
                  Elegir otro tipo
                </button>
              </div>
            </>
          ) : (
            <>
              <h2 className="seccion-titulo">Elige el tipo de problema</h2>
              {motivoSinSugerencia && <p className="mt-1 text-sm text-gris">{motivoSinSugerencia}</p>}
              <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2" role="radiogroup" aria-label="Tipo de problema">
                {CATEGORIAS.map((c) => {
                  const marcada = (elegida ?? categoriaActual?.id_categoria) === c.id_categoria
                  return (
                    <button
                      key={c.id_categoria}
                      type="button"
                      role="radio"
                      aria-checked={marcada}
                      onClick={() => setElegida(c.id_categoria)}
                      className={`flex items-start gap-3 rounded-xl border p-3 text-left ${
                        marcada ? 'border-urbia-800 bg-urbia-50 ring-1 ring-urbia-800' : 'border-linea hover:border-urbia-300'
                      }`}
                    >
                      <c.Icono size={20} strokeWidth={1.75} className="mt-0.5 shrink-0 text-urbia-800" aria-hidden />
                      <span>
                        <span className="block font-semibold">{c.etiqueta}</span>
                        <span className="block text-sm text-gris">{c.descripcion}</span>
                      </span>
                    </button>
                  )
                })}
              </div>
              <button
                type="button"
                className="btn-primario mt-5 w-full"
                disabled={confirmar.isPending || !(elegida ?? categoriaActual?.id_categoria)}
                onClick={() => guardar((elegida ?? categoriaActual?.id_categoria)!)}
              >
                {confirmar.isPending && <Spinner />} Guardar tipo de problema
              </button>
            </>
          )}

          {confirmar.isError && (
            <div className="mt-4">
              <MensajeError error={confirmar.error} />
            </div>
          )}
        </section>
      )}

      <div className="mt-6 flex flex-wrap justify-between gap-3">
        <Link to={`/reportes/${id}`} replace className="btn-texto">
          Ver el reporte
        </Link>
        <Link to="/reportar" className="btn-texto">
          Reportar otro problema
        </Link>
      </div>
    </div>
  )
}
