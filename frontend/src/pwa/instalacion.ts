// Botón "Instalar app": se guarda el evento beforeinstallprompt para mostrarlo cuando el usuario quiera.
import { useSyncExternalStore } from 'react'

interface EventoInstalacion extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

let eventoGuardado: EventoInstalacion | null = null
const oyentes = new Set<() => void>()
const avisar = () => oyentes.forEach((o) => o())

if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault() // se muestra nuestro propio botón
    eventoGuardado = e as EventoInstalacion
    avisar()
  })
  window.addEventListener('appinstalled', () => {
    eventoGuardado = null
    avisar()
  })
}

export const esAppInstalada = () =>
  window.matchMedia('(display-mode: standalone)').matches ||
  (navigator as Navigator & { standalone?: boolean }).standalone === true

const esIOS = () => /iphone|ipad|ipod/i.test(navigator.userAgent)

export function useInstalacion() {
  const evento = useSyncExternalStore(
    (o) => {
      oyentes.add(o)
      return () => oyentes.delete(o)
    },
    () => eventoGuardado,
  )
  return {
    /** Android/Chrome/Edge: se puede abrir el diálogo del sistema. */
    puedeInstalar: evento !== null,
    /** iPhone: no hay diálogo, se explica cómo usar "Agregar a inicio". */
    instruccionesIOS: !evento && esIOS() && !esAppInstalada(),
    instalar: async () => {
      if (!eventoGuardado) return false
      await eventoGuardado.prompt()
      const r = await eventoGuardado.userChoice
      eventoGuardado = null
      avisar()
      return r.outcome === 'accepted'
    },
  }
}

