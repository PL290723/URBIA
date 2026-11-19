// Pantalla principal: mapa de reportes del área visible.
import { useMemo, useState } from 'react'
import { LocateFixed, TriangleAlert } from 'lucide-react'
import { useReportes } from '../api/consultas'
import type { Estado, FiltrosReportes } from '../api/tipos'
import { useEsAdmin } from '../auth/useSesion'
import { MapaReportes, MiUbicacion } from '../components/mapa'
import { Spinner } from '../components/ui'
import { ESTADOS, ESTADO_INFO } from '../lib/catalogos'
import { useGps } from '../lib/geo'
import { mensajeError } from '../api/client'

function Chip({ activo, onClick, color, children }: { activo: boolean; onClick: () => void; color?: string; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={activo}
      className={`flex h-9 shrink-0 items-center gap-2 rounded-full border px-3.5 text-sm font-medium shadow-sm ${
        activo ? 'border-urbia-800 bg-urbia-800 text-white' : 'border-linea bg-white text-asfalto'
      }`}
    >
      {color && <span className="size-2.5 rounded-full ring-2 ring-white/80" style={{ background: color }} aria-hidden />}
      {children}
    </button>
  )
}

export function Inicio() {
  const esAdmin = useEsAdmin()
  const [bbox, setBbox] = useState<string | null>(null)
  const [estados, setEstados] = useState<Estado[]>([])
  const [mios, setMios] = useState(false)
  const gps = useGps()

  const filtros: FiltrosReportes = useMemo(
    () => ({
      bbox: bbox ?? undefined,
      por_pagina: 100,
      estado: estados.length ? estados.join(',') : undefined,
      mios: mios || undefined,
    }),
    [bbox, estados, mios],
  )
  const { data, isFetching, error } = useReportes(filtros, bbox !== null)

  const centro = useMemo<[number, number] | null>(
    () => (gps.estado.tipo === 'listo' ? [gps.estado.lat, gps.estado.lon] : null),
    // Solo recentrar cuando llega la primera lectura, no con cada mejora de precisión
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [gps.estado.tipo],
  )

  const alternar = (e: Estado) => setEstados((xs) => (xs.includes(e) ? xs.filter((x) => x !== e) : [...xs, e]))

  let aviso: { texto: string; tono: 'neutro' | 'alerta' } | null = null
  if (error) aviso = { texto: mensajeError(error), tono: 'alerta' }
  else if (gps.estado.tipo === 'error') aviso = { texto: gps.estado.mensaje, tono: 'alerta' }
  else if (data && data.paginacion.total > data.elementos.length)
    aviso = { texto: `Se ven ${data.elementos.length} de ${data.paginacion.total} reportes. Acerca el mapa para ver el resto.`, tono: 'neutro' }

  return (
    <div className="mapa-inicio absolute inset-0">
      <MapaReportes
        reportes={data?.elementos ?? []}
        onAreaCambia={setBbox}
        centro={centro}
        className="h-full w-full"
        zoomEn="bottomleft"
        extra={
          gps.estado.tipo === 'listo' && (
            <MiUbicacion lat={gps.estado.lat} lon={gps.estado.lon} precision={gps.estado.precision} />
          )
        }
      />

      {/* Filtros por estado */}
      <div className="pointer-events-none absolute inset-x-0 top-0 z-[500] px-3 pt-3">
        <div className="pointer-events-auto -mx-3 flex gap-2 overflow-x-auto px-3 pb-2 [scrollbar-width:none]" role="group" aria-label="Filtrar por estado">
          {!esAdmin && (
            <Chip activo={mios} onClick={() => setMios((m) => !m)}>
              Mis reportes
            </Chip>
          )}
          {ESTADOS.map((e) => (
            <Chip key={e} activo={estados.includes(e)} onClick={() => alternar(e)} color={ESTADO_INFO[e].color}>
              {ESTADO_INFO[e].etiqueta}
            </Chip>
          ))}
        </div>

        {(aviso || isFetching) && (
          <div className="pointer-events-auto mt-1 inline-flex max-w-full items-center gap-2 rounded-[10px] border border-linea bg-white px-3 py-2 text-sm shadow-sm">
            {isFetching && !aviso ? (
              <>
                <Spinner className="size-3.5 text-urbia-600" /> Actualizando reportes
              </>
            ) : (
              <>
                {aviso!.tono === 'alerta' && <TriangleAlert size={16} className="shrink-0 text-amber-600" aria-hidden />}
                <span className={aviso!.tono === 'alerta' ? 'text-amber-950' : 'text-gris'}>{aviso!.texto}</span>
              </>
            )}
          </div>
        )}
      </div>

      <button
        type="button"
        onClick={gps.buscar}
        className="absolute right-3 bottom-24 z-[500] flex size-12 items-center justify-center rounded-full border border-linea bg-white text-urbia-800 shadow-md sm:bottom-6"
        aria-label="Centrar el mapa en mi ubicación"
        title="Mi ubicación"
      >
        {gps.estado.tipo === 'buscando' ? <Spinner className="size-5" /> : <LocateFixed size={22} aria-hidden />}
      </button>

      {!esAdmin && (
        <p className="absolute right-3 bottom-6 z-[500] hidden max-w-60 rounded-[10px] bg-white/95 px-3 py-2 text-xs text-gris shadow-sm sm:right-20 sm:block">
          Los reportes de otras personas aparecen con ubicación aproximada.
        </p>
      )}
    </div>
  )
}
