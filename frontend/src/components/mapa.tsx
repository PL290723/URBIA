// Mapas con Leaflet + OpenStreetMap.
import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import L from 'leaflet'
import { Circle, MapContainer, Marker, Popup, TileLayer, ZoomControl, useMap, useMapEvents } from 'react-leaflet'
import { Link } from 'react-router'
import type { Categoria, Estado } from '../api/tipos'
import { CENTRO_CELAYA } from '../lib/geo'
import { MapPin as nMapPin, type IconNode } from 'lucide'
import { ESTADO_INFO, nodoCategoria, nombreCategoria } from '../lib/catalogos'
import { haceCuanto } from '../lib/formato'

const ATRIBUCION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
const MOSAICOS = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png'

/** Lo mínimo que necesita un punto del mapa (sirve para ReporteResumen y Coincidencia). */
export interface PuntoReporte {
  id_reporte: number
  estado: Estado
  categoria: Categoria | null
  descripcion: string
  fecha_creacion: string
  ubicacion: { latitud: number; longitud: number; aproximada: boolean }
}

/** Convierte un ícono de Lucide (vanilla) en SVG en texto para los divIcon de Leaflet. */
function svgDe(nodo: IconNode, tam: number, color = '#fff'): string {
  const hijos = nodo
    .map(([tag, attrs]) => `<${tag} ${Object.entries(attrs).map(([k, v]) => `${k}="${v}"`).join(' ')}/>`)
    .join('')
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${tam}" height="${tam}" viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${hijos}</svg>`
}

const iconos = new Map<string, L.DivIcon>()
function iconoReporte(estado: Estado, categoria: Categoria | null, resaltado = false): L.DivIcon {
  const clave = `${estado}|${categoria?.nombre ?? ''}|${resaltado}`
  let icono = iconos.get(clave)
  if (!icono) {
    const color = ESTADO_INFO[estado].color
    const tam = resaltado ? 38 : 30
    // Círculo del color del estado con el ícono de la categoría y una punta hacia el lugar exacto
    icono = L.divIcon({
      className: 'marcador-urbia',
      iconSize: [tam, tam + 6],
      iconAnchor: [tam / 2, tam + 6],
      popupAnchor: [0, -tam - 2],
      html: `<div style="position:relative;width:${tam}px;height:${tam + 6}px">
        <div style="width:${tam}px;height:${tam}px;border-radius:50%;background:${color};box-shadow:0 0 0 ${resaltado ? 3 : 2}px ${resaltado ? '#144845' : '#fff'},0 2px 6px rgb(0 0 0 / .3);display:flex;align-items:center;justify-content:center">${svgDe(nodoCategoria(categoria), Math.round(tam * 0.52))}</div>
        <div style="position:absolute;left:50%;bottom:0;width:2px;height:7px;margin-left:-1px;background:${resaltado ? '#144845' : '#fff'}"></div>
      </div>`,
    })
    iconos.set(clave, icono)
  }
  return icono
}

const iconoPin = L.divIcon({
  className: 'marcador-urbia',
  iconSize: [40, 48],
  iconAnchor: [20, 48],
  html: `<div style="position:relative;width:40px;height:48px">
    <div style="width:40px;height:40px;border-radius:50%;background:#f2c230;box-shadow:0 0 0 3px #144845,0 3px 10px rgb(0 0 0 / .35);display:flex;align-items:center;justify-content:center">${svgDe(nMapPin, 22, '#22292a')}</div>
    <div style="position:absolute;left:50%;bottom:0;width:3px;height:9px;margin-left:-1.5px;background:#144845;border-radius:2px"></div>
  </div>`,
})

function Base({
  centro,
  zoom,
  className,
  zoomEn = 'topleft',
  children,
}: {
  centro?: [number, number]
  zoom?: number
  className: string
  zoomEn?: L.ControlPosition
  children: ReactNode
}) {
  return (
    <MapContainer center={centro ?? CENTRO_CELAYA} zoom={zoom ?? 14} className={className} scrollWheelZoom zoomControl={false}>
      <TileLayer attribution={ATRIBUCION} url={MOSAICOS} maxZoom={19} />
      <ZoomControl position={zoomEn} />
      {children}
    </MapContainer>
  )
}

/** Avisa el rectángulo visible en formato bbox de la API: min_lon,min_lat,max_lon,max_lat. */
function VigilarArea({ onCambio }: { onCambio: (bbox: string) => void }) {
  const espera = useRef<number | null>(null)
  const avisar = (m: L.Map) => {
    if (espera.current) window.clearTimeout(espera.current)
    espera.current = window.setTimeout(() => {
      const b = m.getBounds()
      const r = (n: number) => n.toFixed(5)
      onCambio(`${r(b.getWest())},${r(b.getSouth())},${r(b.getEast())},${r(b.getNorth())}`)
    }, 350)
  }
  const mapa = useMapEvents({ moveend: () => avisar(mapa) })
  useEffect(() => {
    avisar(mapa)
    return () => {
      if (espera.current) window.clearTimeout(espera.current)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mapa])
  return null
}

function Recentrar({ centro, zoom }: { centro: [number, number] | null; zoom?: number }) {
  const mapa = useMap()
  useEffect(() => {
    if (centro) mapa.setView(centro, zoom ?? Math.max(mapa.getZoom(), 16))
  }, [centro, zoom, mapa])
  return null
}

export function MapaReportes({
  reportes,
  onAreaCambia,
  centro,
  resaltado,
  className = 'h-full w-full',
  zoomEn,
  extra,
}: {
  reportes: PuntoReporte[]
  onAreaCambia?: (bbox: string) => void
  centro?: [number, number] | null
  resaltado?: number
  className?: string
  zoomEn?: L.ControlPosition
  extra?: ReactNode
}) {
  return (
    <Base className={className} centro={centro ?? undefined} zoom={centro ? 16 : undefined} zoomEn={zoomEn}>
      {onAreaCambia && <VigilarArea onCambio={onAreaCambia} />}
      {reportes.map((r) => (
        <Marker
          key={r.id_reporte}
          position={[r.ubicacion.latitud, r.ubicacion.longitud]}
          icon={iconoReporte(r.estado, r.categoria, r.id_reporte === resaltado)}
          zIndexOffset={r.id_reporte === resaltado ? 1000 : 0}
        >
          <Popup>
            <div className="w-56">
              <div className="flex items-center justify-between gap-2">
                <p className="font-semibold text-asfalto">{nombreCategoria(r.categoria)}</p>
                <span className="text-xs text-gris">{haceCuanto(r.fecha_creacion)}</span>
              </div>
              <p className="mt-1 line-clamp-3 text-sm text-gris">{r.descripcion}</p>
              <div className="mt-2 flex items-center justify-between gap-2">
                <span className="text-xs font-medium" style={{ color: ESTADO_INFO[r.estado].color }}>
                  {ESTADO_INFO[r.estado].etiqueta}
                </span>
                <Link to={`/reportes/${r.id_reporte}`} className="text-sm font-semibold text-urbia-700">
                  Ver reporte
                </Link>
              </div>
              {r.ubicacion.aproximada && <p className="mt-1.5 text-[11px] text-gris-claro">Ubicación aproximada para proteger a quien reportó</p>}
            </div>
          </Popup>
        </Marker>
      ))}
      {extra}
    </Base>
  )
}

/** Círculo de "aquí estás" (precisión del GPS). */
export function MiUbicacion({ lat, lon, precision }: { lat: number; lon: number; precision: number }) {
  return (
    <>
      <Circle center={[lat, lon]} radius={precision} pathOptions={{ color: '#144845', weight: 1, fillOpacity: 0.08 }} />
      <Circle center={[lat, lon]} radius={6} pathOptions={{ color: '#fff', weight: 3, fillColor: '#144845', fillOpacity: 1 }} />
    </>
  )
}

function ClicParaMover({ onMover }: { onMover: (lat: number, lon: number) => void }) {
  useMapEvents({ click: (e) => onMover(e.latlng.lat, e.latlng.lng) })
  return null
}

/** Mapa para elegir dónde está el problema: tocar el mapa o arrastrar el pin. */
export function SelectorUbicacion({
  punto,
  precision,
  onCambio,
  className = 'h-72 w-full',
}: {
  punto: [number, number] | null
  precision?: number
  onCambio: (lat: number, lon: number) => void
  className?: string
}) {
  const eventos = useMemo(
    () => ({
      dragend(e: L.DragEndEvent) {
        const p = (e.target as L.Marker).getLatLng()
        onCambio(p.lat, p.lng)
      },
    }),
    [onCambio],
  )
  return (
    <Base className={className} centro={punto ?? undefined} zoom={punto ? 17 : 14}>
      <ClicParaMover onMover={onCambio} />
      <Recentrar centro={punto} />
      {punto && precision && (
        <Circle center={punto} radius={precision} pathOptions={{ color: '#144845', weight: 1, fillOpacity: 0.1 }} />
      )}
      {punto && <Marker position={punto} icon={iconoPin} draggable eventHandlers={eventos} />}
    </Base>
  )
}

/** Mapa de solo lectura de un punto (detalle de un reporte). */
export function MapaPunto({
  lat,
  lon,
  aproximada,
  estado,
  categoria,
  className = 'h-56 w-full',
  children,
}: {
  lat: number
  lon: number
  aproximada: boolean
  estado: Estado
  categoria: Categoria | null
  className?: string
  children?: ReactNode
}) {
  return (
    <Base className={className} centro={[lat, lon]} zoom={aproximada ? 16 : 17}>
      {aproximada ? (
        <Circle center={[lat, lon]} radius={110} pathOptions={{ color: ESTADO_INFO[estado].color, fillOpacity: 0.15 }} />
      ) : (
        <Marker position={[lat, lon]} icon={iconoReporte(estado, categoria, true)} />
      )}
      {children}
    </Base>
  )
}

