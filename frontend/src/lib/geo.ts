import { useCallback, useEffect, useRef, useState } from 'react'

export const CENTRO_CELAYA: [number, number] = [
  Number(import.meta.env.VITE_MAPA_LAT ?? 20.5235),
  Number(import.meta.env.VITE_MAPA_LON ?? -100.8157),
]

/** La API solo acepta precision_m ≤ 100 con origen "gps"; si es peor, se pide marcar en el mapa. */
export const PRECISION_MAXIMA_M = 100

export type EstadoGps =
  | { tipo: 'inactivo' }
  | { tipo: 'buscando' }
  | { tipo: 'listo'; lat: number; lon: number; precision: number }
  | { tipo: 'error'; mensaje: string }

/** Contexto seguro = https o localhost. Sin él el navegador bloquea el GPS. */
export const gpsDisponible = typeof navigator !== 'undefined' && 'geolocation' in navigator && window.isSecureContext

function mensajeGps(e: GeolocationPositionError): string {
  switch (e.code) {
    case e.PERMISSION_DENIED:
      return 'Permiso de ubicación denegado. Actívalo en el navegador o marca el punto en el mapa.'
    case e.POSITION_UNAVAILABLE:
      return 'No hay señal de GPS. Marca el punto en el mapa.'
    case e.TIMEOUT:
      return 'El GPS tardó demasiado. Intenta de nuevo o marca el punto en el mapa.'
    default:
      return 'No se pudo obtener la ubicación.'
  }
}

/**
 * Observa el GPS unos segundos y se queda con la lectura más precisa
 * (la primera suele ser la de la red celular, de cientos de metros).
 */
export function useGps() {
  const [estado, setEstado] = useState<EstadoGps>({ tipo: 'inactivo' })
  const vigilancia = useRef<number | null>(null)
  const temporizador = useRef<number | null>(null)

  const detener = useCallback(() => {
    if (vigilancia.current !== null) navigator.geolocation.clearWatch(vigilancia.current)
    if (temporizador.current !== null) window.clearTimeout(temporizador.current)
    vigilancia.current = null
    temporizador.current = null
  }, [])

  const buscar = useCallback(() => {
    if (!gpsDisponible) {
      setEstado({
        tipo: 'error',
        mensaje: window.isSecureContext
          ? 'Este navegador no tiene GPS. Marca el punto en el mapa.'
          : 'El GPS solo funciona con HTTPS. Marca el punto en el mapa.',
      })
      return
    }
    detener()
    setEstado({ tipo: 'buscando' })
    let mejor: GeolocationPosition | null = null

    vigilancia.current = navigator.geolocation.watchPosition(
      (pos) => {
        if (!mejor || pos.coords.accuracy < mejor.coords.accuracy) {
          mejor = pos
          setEstado({
            tipo: 'listo',
            lat: pos.coords.latitude,
            lon: pos.coords.longitude,
            precision: Math.max(1, Math.round(pos.coords.accuracy * 10) / 10),
          })
        }
        if (pos.coords.accuracy <= 15) detener()
      },
      (err) => {
        if (!mejor) setEstado({ tipo: 'error', mensaje: mensajeGps(err) })
        detener()
      },
      { enableHighAccuracy: true, timeout: 20000, maximumAge: 0 },
    )
    // Máximo 20 s buscando una lectura mejor
    temporizador.current = window.setTimeout(detener, 20000)
  }, [detener])

  useEffect(() => detener, [detener])

  return { estado, buscar, detener }
}
