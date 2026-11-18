// Panel administrativo: estadísticas agregadas (GET /api/v1/estadisticas).
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  LabelList,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Circle, MapContainer, TileLayer, Tooltip as TooltipMapa } from 'react-leaflet'
import { useEstadisticas, useSalud } from '../../api/consultas'
import type { Estado, Estadisticas, FiltrosEstadisticas } from '../../api/tipos'
import { Cargando, Encabezado, MensajeError } from '../../components/ui'
import { ESTADOS, ESTADO_INFO, nombreCategoria } from '../../lib/catalogos'
import { fecha, diaLocal } from '../../lib/formato'
import { CENTRO_CELAYA } from '../../lib/geo'

const TINTA = '#22292a'
const TINTA_SUAVE = '#5b6664'
const REJILLA = '#e3e8e5'
const SERIE = '#1d564f' // urbia-700: una sola serie, un solo tono

const RANGOS = [
  { dias: 7, texto: 'Semana' },
  { dias: 30, texto: '30 días' },
  { dias: 90, texto: '3 meses' },
  { dias: 365, texto: 'Año' },
]

function Seccion({ titulo, descripcion, children, className = '' }: { titulo: string; descripcion?: string; children: React.ReactNode; className?: string }) {
  return (
    <section className={`hoja p-5 ${className}`}>
      <h2 className="seccion-titulo">{titulo}</h2>
      {descripcion && <p className="text-sm text-gris">{descripcion}</p>}
      <div className="mt-4">{children}</div>
    </section>
  )
}

/** Franja de conteos: total y uno por estado, cada uno lleva a la bandeja filtrada. */
function Conteos({ total, porEstado }: { total: number; porEstado: { estado: Estado; nombre: string; total: number }[] }) {
  return (
    <div className="hoja grid grid-cols-2 divide-linea overflow-hidden sm:grid-cols-5 sm:divide-x">
      <div className="col-span-2 border-b border-linea bg-urbia-800 p-5 text-white sm:col-span-1 sm:border-b-0">
        <p className="text-sm text-urbia-100">Recibidos en el periodo</p>
        <p className="titulo mt-1 text-5xl text-white tabular-nums">{total.toLocaleString('es-MX')}</p>
      </div>
      {porEstado.map((e, i) => (
        <Link
          key={e.estado}
          to={`/admin/reportes?estado=${e.estado}`}
          className={`group p-5 hover:bg-concreto ${i < 2 ? 'max-sm:border-b max-sm:border-linea' : ''} ${i % 2 === 0 ? 'max-sm:border-r max-sm:border-linea' : ''}`}
        >
          <p className="flex items-center gap-2 text-sm text-gris">
            <span className="size-2.5 rounded-full" style={{ background: ESTADO_INFO[e.estado].color }} aria-hidden />
            {e.nombre}
          </p>
          <p className="titulo mt-1 text-4xl tabular-nums group-hover:text-urbia-800">{e.total.toLocaleString('es-MX')}</p>
        </Link>
      ))}
    </div>
  )
}

function TooltipSimple({ active, payload, label, unidad = 'reportes' }: {
  active?: boolean
  payload?: { value?: number | string }[]
  label?: string | number
  unidad?: string
}) {
  if (!active || !payload?.length) return null
  return (
    <div className="rounded-[10px] border border-linea bg-white px-3 py-2 text-sm shadow-md">
      <p className="font-semibold">{label}</p>
      <p className="text-gris tabular-nums">
        {Number(payload[0].value).toLocaleString('es-MX')} {unidad}
      </p>
    </div>
  )
}

function TablaDatos({ filas, columnas }: { filas: (string | number)[][]; columnas: string[] }) {
  return (
    <details className="mt-3 text-sm">
      <summary className="cursor-pointer text-gris hover:text-asfalto">Ver los datos en tabla</summary>
      <table className="mt-2 w-full">
        <thead>
          <tr className="text-left text-gris">
            {columnas.map((c) => (
              <th key={c} className="py-1 font-medium">
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {filas.map((f, i) => (
            <tr key={i} className="border-t border-linea">
              {f.map((v, j) => (
                <td key={j} className="py-1 tabular-nums">
                  {v}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </details>
  )
}

function etiquetaPeriodo(inicio: string, agrupacion: Estadisticas['filtros']['agrupacion']) {
  const d = new Date(`${inicio}T12:00:00`)
  if (agrupacion === 'mes') return d.toLocaleDateString('es-MX', { month: 'short', year: '2-digit' })
  return d.toLocaleDateString('es-MX', { day: 'numeric', month: 'short' })
}

function MapaCalor({ geo }: { geo: NonNullable<Estadisticas['distribucion_geografica']> }) {
  const max = Math.max(1, ...geo.celdas.map((c) => c.total))
  const centro: [number, number] = geo.celdas.length
    ? [
        geo.celdas.reduce((s, c) => s + c.latitud, 0) / geo.celdas.length,
        geo.celdas.reduce((s, c) => s + c.longitud, 0) / geo.celdas.length,
      ]
    : CENTRO_CELAYA
  // Radio ≈ media celda (1° de latitud ≈ 111 km)
  const radioBase = (geo.celda_grados * 111_000) / 2
  return (
    <>
      <MapContainer center={centro} zoom={13} className="h-96 w-full rounded-xl" scrollWheelZoom={false}>
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        {geo.celdas.map((c) => {
          const t = c.total / max
          return (
            <Circle
              key={`${c.latitud},${c.longitud}`}
              center={[c.latitud, c.longitud]}
              radius={radioBase * (0.45 + 0.55 * Math.sqrt(t))}
              pathOptions={{ color: '#fff', weight: 2, fillColor: SERIE, fillOpacity: 0.25 + 0.6 * t }}
            >
              <TooltipMapa>{c.total} reportes</TooltipMapa>
            </Circle>
          )
        })}
      </MapContainer>
      <p className="mt-3 text-sm text-gris">
        Por privacidad no se muestran las zonas con menos de {geo.minimo_por_celda} reportes ({geo.reportes_ocultos} reportes en{' '}
        {geo.celdas_ocultas} zonas).
      </p>
    </>
  )
}

function EstadoSistema() {
  const { data, isError } = useSalud()
  if (isError) return <span className="text-red-700">La API no responde</span>
  if (!data) return null
  const ia = data.dependencias.inferencia.estado
  return (
    <span className="flex flex-wrap gap-x-4 gap-y-1">
      <span>
        Base de datos{' '}
        <strong className={data.dependencias.database.estado === 'activo' ? 'text-green-800' : 'text-red-700'}>
          {data.dependencias.database.estado === 'activo' ? 'en línea' : 'sin respuesta'}
        </strong>
      </span>
      <span>
        Análisis de fotos{' '}
        <strong className={ia === 'activo' ? 'text-green-800' : 'text-amber-700'}>
          {ia === 'activo' ? 'en línea' : ia === 'no_configurado' ? 'sin configurar' : 'sin servicio'}
        </strong>
      </span>
    </span>
  )
}

export function Panel() {
  const [dias, setDias] = useState(30)
  const [geo, setGeo] = useState(true)
  const [hoy] = useState(() => new Date())
  const filtros: FiltrosEstadisticas = useMemo(
    () => ({ desde: diaLocal(new Date(hoy.getTime() - (dias - 1) * 86400000)), hasta: diaLocal(hoy), geo }),
    [dias, geo, hoy],
  )
  const { data, isPending, isError, error, refetch, isFetching } = useEstadisticas(filtros)

  const porEstado = useMemo(
    () =>
      ESTADOS.map((e) => ({
        estado: e,
        nombre: ESTADO_INFO[e].etiqueta,
        total: data?.por_estado.find((x) => x.estado === e)?.total ?? 0,
      })),
    [data],
  )
  const porCategoria = useMemo(
    () =>
      [...(data?.por_categoria ?? [])]
        .sort((a, b) => b.total - a.total)
        .map((c) => ({ ...c, etiqueta: nombreCategoria(c.id_categoria ? c : null) })),
    [data],
  )
  const porPeriodo = useMemo(
    () =>
      (data?.por_periodo ?? []).map((p) => ({
        ...p,
        etiqueta: etiquetaPeriodo(p.inicio, data!.filtros.agrupacion),
      })),
    [data],
  )

  return (
    <div>
      <Encabezado
        titulo="Resumen"
        subtitulo={<EstadoSistema />}
        acciones={
          <Link to="/admin/reportes?estado=pendiente" className="btn-primario">
            Revisar pendientes
          </Link>
        }
      />

      {/* Filtros en una sola fila */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="inline-flex rounded-[10px] border border-linea bg-white p-1" role="group" aria-label="Periodo">
          {RANGOS.map((r) => (
            <button
              key={r.dias}
              type="button"
              onClick={() => setDias(r.dias)}
              aria-pressed={dias === r.dias}
              className={`h-9 rounded-[7px] px-3 text-sm font-medium ${
                dias === r.dias ? 'bg-urbia-800 text-white' : 'text-gris hover:text-asfalto'
              }`}
            >
              {r.texto}
            </button>
          ))}
        </div>
        <label className="inline-flex items-center gap-2 text-sm text-gris">
          <input type="checkbox" checked={geo} onChange={(e) => setGeo(e.target.checked)} className="accent-urbia-700" />
          Distribución geográfica
        </label>
        {data && (
          <span className="text-sm text-gris sm:ml-auto">
            Del {fecha(`${data.filtros.desde}T12:00:00`)} al {fecha(`${data.filtros.hasta}T12:00:00`)}
          </span>
        )}
      </div>

      {isPending ? (
        <Cargando />
      ) : isError ? (
        <MensajeError error={error} reintentar={refetch} />
      ) : (
        <div className={`space-y-4 ${isFetching ? 'opacity-70' : ''}`}>
          <Conteos total={data.total} porEstado={porEstado} />

          <div className="grid gap-4 xl:grid-cols-2">
            <Seccion titulo="Reportes recibidos" descripcion={`Por ${data.filtros.agrupacion === 'dia' ? 'día' : data.filtros.agrupacion}`}>
              {porPeriodo.length === 0 ? (
                <p className="py-10 text-center text-sm text-gris">No llegaron reportes en este periodo.</p>
              ) : (
                <ResponsiveContainer width="100%" height={260}>
                  <AreaChart data={porPeriodo} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
                    <defs>
                      <linearGradient id="relleno" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor={SERIE} stopOpacity={0.18} />
                        <stop offset="100%" stopColor={SERIE} stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid stroke={REJILLA} vertical={false} />
                    <XAxis dataKey="etiqueta" tick={{ fill: TINTA_SUAVE, fontSize: 12 }} tickLine={false} axisLine={{ stroke: REJILLA }} minTickGap={20} />
                    <YAxis allowDecimals={false} tick={{ fill: TINTA_SUAVE, fontSize: 12 }} tickLine={false} axisLine={false} />
                    <Tooltip content={<TooltipSimple />} cursor={{ stroke: TINTA_SUAVE, strokeDasharray: '3 3' }} />
                    <Area type="monotone" dataKey="total" stroke={SERIE} strokeWidth={2} fill="url(#relleno)" activeDot={{ r: 5, stroke: '#fff', strokeWidth: 2 }} />
                  </AreaChart>
                </ResponsiveContainer>
              )}
              <TablaDatos columnas={['Periodo', 'Reportes']} filas={porPeriodo.map((p) => [p.inicio, p.total])} />
            </Seccion>

            <Seccion titulo="Por tipo de problema" descripcion="De más a menos frecuente">
              {porCategoria.length === 0 ? (
                <p className="py-10 text-center text-sm text-gris">No llegaron reportes en este periodo.</p>
              ) : (
                <ResponsiveContainer width="100%" height={Math.max(160, porCategoria.length * 32)}>
                  <BarChart data={porCategoria} layout="vertical" margin={{ top: 0, right: 36, left: 0, bottom: 0 }} barCategoryGap={6}>
                    <XAxis type="number" hide allowDecimals={false} />
                    <YAxis type="category" dataKey="etiqueta" width={130} tick={{ fill: TINTA, fontSize: 13 }} tickLine={false} axisLine={false} />
                    <Tooltip content={<TooltipSimple />} cursor={{ fill: '#f2f4f1' }} />
                    <Bar dataKey="total" fill={SERIE} radius={[0, 4, 4, 0]}>
                      <LabelList dataKey="total" position="right" fill={TINTA} fontSize={13} />
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              )}
              <TablaDatos
                columnas={['Tipo', 'Total', ...ESTADOS.map((e) => ESTADO_INFO[e].etiqueta)]}
                filas={porCategoria.map((c) => [c.etiqueta, c.total, ...ESTADOS.map((e) => c.por_estado[e] ?? 0)])}
              />
            </Seccion>
          </div>

          {data.distribucion_geografica && (
            <Seccion titulo="Dónde se concentran" descripcion="Zonas con más reportes en el periodo; el tono más oscuro indica más reportes.">
              <MapaCalor geo={data.distribucion_geografica} />
            </Seccion>
          )}
        </div>
      )}
    </div>
  )
}
