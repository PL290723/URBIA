// Bandeja de reportes del administrador. Los filtros viven en la URL para poder compartir la vista.
import { useMemo } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { ChevronRight, Inbox, X } from 'lucide-react'
import { useReportes } from '../../api/consultas'
import type { Estado, FiltrosReportes } from '../../api/tipos'
import {
  Cargando,
  Encabezado,
  IconoCategoria,
  InsigniaEstado,
  InsigniaPrioridad,
  MensajeError,
  Paginador,
  Vacio,
} from '../../components/ui'
import { CATEGORIAS, ESTADOS, ESTADO_INFO, nombreCategoria } from '../../lib/catalogos'
import { fechaHora, haceCuanto } from '../../lib/formato'

export function AdminReportes() {
  const [params, setParams] = useSearchParams()
  const navegar = useNavigate()

  const estados = (params.get('estado')?.split(',').filter(Boolean) ?? []) as Estado[]
  const filtros: FiltrosReportes = useMemo(
    () => ({
      pagina: Number(params.get('pagina')) || 1,
      por_pagina: 20,
      estado: params.get('estado') || undefined,
      id_categoria: Number(params.get('categoria')) || undefined,
      desde: params.get('desde') || undefined,
      hasta: params.get('hasta') || undefined,
    }),
    [params],
  )
  const { data, isPending, isError, error, refetch, isFetching } = useReportes(filtros)

  const cambiar = (clave: string, valor: string | null) => {
    const p = new URLSearchParams(params)
    if (valor) p.set(clave, valor)
    else p.delete(clave)
    if (clave !== 'pagina') p.delete('pagina')
    setParams(p, { replace: true })
  }

  const alternarEstado = (e: Estado) => {
    const nuevos = estados.includes(e) ? estados.filter((x) => x !== e) : [...estados, e]
    cambiar('estado', nuevos.join(','))
  }

  const hayFiltros = ['estado', 'categoria', 'desde', 'hasta'].some((k) => params.get(k))

  return (
    <div>
      <Encabezado
        titulo="Reportes"
        subtitulo={data ? `${data.paginacion.total} ${hayFiltros ? 'con estos filtros' : 'en total'}` : 'Bandeja de reportes ciudadanos'}
      />

      {/* Filtros: una sola franja */}
      <div className="mb-5 flex flex-wrap items-end gap-x-4 gap-y-3">
        <div>
          <span className="etiqueta">Estado</span>
          <div className="-mx-4 flex gap-1.5 overflow-x-auto px-4 [scrollbar-width:none] sm:mx-0 sm:flex-wrap sm:px-0" role="group" aria-label="Estado">
            {ESTADOS.map((e) => {
              const activo = estados.includes(e)
              return (
                <button
                  key={e}
                  type="button"
                  aria-pressed={activo}
                  onClick={() => alternarEstado(e)}
                  className={`flex h-11 shrink-0 items-center gap-2 rounded-[10px] border px-3 text-sm font-medium ${
                    activo ? 'border-urbia-800 bg-urbia-800 text-white' : 'border-linea bg-white hover:border-urbia-300'
                  }`}
                >
                  <span className="size-2.5 rounded-full ring-2 ring-white/70" style={{ background: ESTADO_INFO[e].color }} aria-hidden />
                  {ESTADO_INFO[e].etiqueta}
                </button>
              )
            })}
          </div>
        </div>
        <label className="w-full sm:w-48">
          <span className="etiqueta">Tipo de problema</span>
          <select className="campo" value={params.get('categoria') ?? ''} onChange={(e) => cambiar('categoria', e.target.value)}>
            <option value="">Todos</option>
            {CATEGORIAS.map((c) => (
              <option key={c.id_categoria} value={c.id_categoria}>
                {c.etiqueta}
              </option>
            ))}
          </select>
        </label>
        <label className="flex-1 sm:w-40 sm:flex-none">
          <span className="etiqueta">Desde</span>
          <input type="date" className="campo" value={params.get('desde') ?? ''} onChange={(e) => cambiar('desde', e.target.value)} />
        </label>
        <label className="flex-1 sm:w-40 sm:flex-none">
          <span className="etiqueta">Hasta</span>
          <input type="date" className="campo" value={params.get('hasta') ?? ''} onChange={(e) => cambiar('hasta', e.target.value)} />
        </label>
        {hayFiltros && (
          <button type="button" className="btn-texto h-11 text-sm" onClick={() => setParams({}, { replace: true })}>
            <X size={16} aria-hidden /> Quitar filtros
          </button>
        )}
      </div>

      {isPending ? (
        <Cargando />
      ) : isError ? (
        <MensajeError error={error} reintentar={refetch} />
      ) : data.elementos.length === 0 ? (
        <Vacio Icono={Inbox} titulo={hayFiltros ? 'Ningún reporte coincide con los filtros' : 'Aún no llegan reportes'} />
      ) : (
        <div className={isFetching ? 'opacity-60' : ''}>
          {/* Escritorio: tabla. La franja de color a la izquierda es el estado. */}
          <div className="hoja hidden overflow-hidden md:block">
            <table className="w-full text-sm">
              <thead className="border-b border-linea text-left text-gris">
                <tr>
                  <th className="py-3 pr-3 pl-5 font-medium">Problema</th>
                  <th className="px-3 py-3 font-medium">Estado</th>
                  <th className="px-3 py-3 font-medium">Prioridad</th>
                  <th className="px-3 py-3 font-medium">Reportó</th>
                  <th className="px-3 py-3 text-right font-medium">Recibido</th>
                  <th className="w-10" />
                </tr>
              </thead>
              <tbody className="divide-y divide-linea">
                {data.elementos.map((r) => (
                  <tr
                    key={r.id_reporte}
                    onClick={() => navegar(`/reportes/${r.id_reporte}`)}
                    className="group cursor-pointer hover:bg-concreto"
                    style={{ boxShadow: `inset 3px 0 0 ${ESTADO_INFO[r.estado].color}` }}
                  >
                    <td className="py-3 pr-3 pl-5">
                      <div className="flex items-center gap-3">
                        <IconoCategoria categoria={r.categoria} tamano="sm" />
                        <div className="min-w-0">
                          <Link
                            to={`/reportes/${r.id_reporte}`}
                            onClick={(e) => e.stopPropagation()}
                            className="font-semibold group-hover:text-urbia-800"
                          >
                            {nombreCategoria(r.categoria)}
                          </Link>
                          <span className="ml-2 text-xs text-gris-claro tabular-nums">{r.id_reporte}</span>
                          <p className="max-w-md truncate text-gris">{r.descripcion}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-3">
                      <InsigniaEstado estado={r.estado} />
                    </td>
                    <td className="px-3 py-3">
                      <InsigniaPrioridad nivel={r.administrativo?.prioridad} />
                    </td>
                    <td className="max-w-40 truncate px-3 py-3 text-gris">{r.administrativo?.autor.nombre ?? 'Sin dato'}</td>
                    <td className="px-3 py-3 text-right whitespace-nowrap text-gris" title={fechaHora(r.fecha_creacion)}>
                      {haceCuanto(r.fecha_creacion)}
                    </td>
                    <td className="pr-3">
                      <ChevronRight size={18} className="text-gris-claro group-hover:text-urbia-700" aria-hidden />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Celular: lista */}
          <ul className="hoja divide-y divide-linea overflow-hidden md:hidden">
            {data.elementos.map((r) => (
              <li key={r.id_reporte} style={{ boxShadow: `inset 3px 0 0 ${ESTADO_INFO[r.estado].color}` }}>
                <Link to={`/reportes/${r.id_reporte}`} className="flex gap-3 py-3 pr-3 pl-4">
                  <IconoCategoria categoria={r.categoria} tamano="sm" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-semibold">{nombreCategoria(r.categoria)}</span>
                      <InsigniaPrioridad nivel={r.administrativo?.prioridad} conTexto={false} />
                    </div>
                    <p className="mt-0.5 line-clamp-1 text-sm text-gris">{r.descripcion}</p>
                    <div className="mt-1.5 flex items-center justify-between gap-2">
                      <InsigniaEstado estado={r.estado} />
                      <span className="text-xs text-gris">{haceCuanto(r.fecha_creacion)}</span>
                    </div>
                  </div>
                </Link>
              </li>
            ))}
          </ul>

          <Paginador
            pagina={data.paginacion.pagina}
            totalPaginas={data.paginacion.total_paginas}
            total={data.paginacion.total}
            onCambiar={(p) => cambiar('pagina', String(p))}
          />
        </div>
      )}
    </div>
  )
}
