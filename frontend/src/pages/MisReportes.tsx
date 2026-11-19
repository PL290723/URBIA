import { useState } from 'react'
import { Link } from 'react-router'
import { Camera, ChevronRight, ClipboardList } from 'lucide-react'
import { useReportes } from '../api/consultas'
import type { Estado, ReporteResumen } from '../api/tipos'
import { FotoProtegida } from '../components/FotoProtegida'
import { Cargando, Encabezado, IconoCategoria, InsigniaEstado, MensajeError, Paginador, Vacio } from '../components/ui'
import { ESTADOS, ESTADO_INFO, nombreCategoria } from '../lib/catalogos'
import { haceCuanto } from '../lib/formato'

/** Fila de la lista: foto a la izquierda, texto y estado a la derecha. */
export function FilaReporte({ r }: { r: ReporteResumen }) {
  return (
    <Link to={`/reportes/${r.id_reporte}`} className="group flex items-center gap-3 px-3 py-3 hover:bg-concreto sm:gap-4 sm:px-4">
      {r.id_foto ? (
        <FotoProtegida idArchivo={r.id_foto} alt="" enlace={false} className="size-16 shrink-0 rounded-xl sm:size-20" />
      ) : (
        <span className="flex size-16 shrink-0 items-center justify-center rounded-xl bg-concreto sm:size-20">
          <IconoCategoria categoria={r.categoria} />
        </span>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <p className="font-semibold">{nombreCategoria(r.categoria)}</p>
          <InsigniaEstado estado={r.estado} />
        </div>
        <p className="mt-1 line-clamp-2 text-sm text-gris">{r.descripcion}</p>
        <p className="mt-1 text-xs text-gris-claro">
          Folio {r.id_reporte}, {haceCuanto(r.fecha_creacion)}
        </p>
      </div>
      <ChevronRight size={20} className="shrink-0 text-gris-claro group-hover:text-urbia-700" aria-hidden />
    </Link>
  )
}

export function MisReportes() {
  const [pagina, setPagina] = useState(1)
  const [estado, setEstado] = useState<Estado | ''>('')
  const { data, isPending, isError, error, refetch, isFetching } = useReportes({
    mios: true,
    pagina,
    por_pagina: 20,
    estado: estado || undefined,
  })

  return (
    <div className="mx-auto max-w-3xl">
      <Encabezado titulo="Mis reportes" subtitulo="El avance de cada problema que reportaste." />

      {/* Pestañas por estado */}
      <div className="-mx-4 mb-4 overflow-x-auto border-b border-linea px-4 [scrollbar-width:none]">
        <div className="flex gap-5" role="tablist" aria-label="Filtrar por estado">
          {(['', ...ESTADOS] as const).map((e) => (
            <button
              key={e || 'todos'}
              type="button"
              role="tab"
              aria-selected={estado === e}
              onClick={() => {
                setEstado(e)
                setPagina(1)
              }}
              className={`-mb-px shrink-0 border-b-2 py-2.5 text-sm font-medium whitespace-nowrap ${
                estado === e ? 'border-urbia-800 text-urbia-800' : 'border-transparent text-gris hover:text-asfalto'
              }`}
            >
              {e ? ESTADO_INFO[e].etiqueta : 'Todos'}
            </button>
          ))}
        </div>
      </div>

      {isPending ? (
        <Cargando />
      ) : isError ? (
        <MensajeError error={error} reintentar={refetch} />
      ) : data.elementos.length === 0 ? (
        <Vacio Icono={ClipboardList} titulo={estado ? `No tienes reportes en “${ESTADO_INFO[estado].etiqueta}”` : 'Todavía no has reportado nada'}>
          {!estado && (
            <Link to="/reportar" className="btn-reportar mt-2">
              <Camera size={18} aria-hidden /> Reportar un problema
            </Link>
          )}
        </Vacio>
      ) : (
        <div className={isFetching ? 'opacity-60' : ''}>
          <ul className="hoja divide-y divide-linea overflow-hidden">
            {data.elementos.map((r) => (
              <li key={r.id_reporte}>
                <FilaReporte r={r} />
              </li>
            ))}
          </ul>
          <Paginador
            pagina={data.paginacion.pagina}
            totalPaginas={data.paginacion.total_paginas}
            total={data.paginacion.total}
            onCambiar={setPagina}
          />
        </div>
      )}
    </div>
  )
}
