// Todo lo relacionado con la PWA: registro del service worker, aviso de nueva versión,
// botón "Instalar app" y aviso de sin conexión.
import { useState, useSyncExternalStore } from 'react'
import { useRegisterSW } from 'virtual:pwa-register/react'
import { Download, RefreshCw, Share, Smartphone, WifiOff, X } from 'lucide-react'
import { useInstalacion } from './instalacion'

// ------------------------------------------------- Aviso de actualización
export function AvisoActualizacion() {
  const {
    needRefresh: [hayVersion, setHayVersion],
    updateServiceWorker,
  } = useRegisterSW({
    onRegisteredSW(_url, registro) {
      // Revisa cada hora si hay una versión nueva (la app instalada puede quedar abierta)
      if (registro) setInterval(() => registro.update(), 60 * 60 * 1000)
    },
  })

  if (!hayVersion) return null
  return (
    <div className="fixed inset-x-3 bottom-20 z-[1100] mx-auto flex max-w-md items-center gap-3 rounded-2xl bg-urbia-800 p-3 pl-4 text-white shadow-xl sm:bottom-4" role="status">
      <RefreshCw size={20} className="shrink-0 text-urbia-200" aria-hidden />
      <p className="flex-1 text-sm">Hay una versión nueva de URBIA.</p>
      <button type="button" className="btn min-h-9 bg-white px-3 text-urbia-800" onClick={() => updateServiceWorker(true)}>
        Actualizar
      </button>
      <button type="button" className="rounded-lg p-2 text-urbia-100 hover:bg-urbia-700" onClick={() => setHayVersion(false)} aria-label="Cerrar aviso">
        <X size={18} aria-hidden />
      </button>
    </div>
  )
}

// ----------------------------------------------------------- Instalación
export function BotonInstalar({ className = '' }: { className?: string }) {
  const { puedeInstalar, instruccionesIOS, instalar } = useInstalacion()
  const [verIOS, setVerIOS] = useState(false)

  if (puedeInstalar) {
    return (
      <button type="button" className={`btn-secundario ${className}`} onClick={instalar}>
        <Download size={18} aria-hidden /> Instalar en este dispositivo
      </button>
    )
  }
  if (instruccionesIOS) {
    return (
      <div className={className}>
        <button type="button" className="btn-secundario w-full" onClick={() => setVerIOS((v) => !v)}>
          <Smartphone size={18} aria-hidden /> Instalar en iPhone
        </button>
        {verIOS && (
          <p className="mt-2 rounded-xl bg-urbia-50 p-3 text-sm text-urbia-900">
            En Safari toca <Share size={14} className="inline align-[-2px]" aria-label="Compartir" /> Compartir y luego
            “Agregar a pantalla de inicio”.
          </p>
        )}
      </div>
    )
  }
  return null
}

// --------------------------------------------------------- Sin conexión
function useEnLinea() {
  return useSyncExternalStore(
    (o) => {
      window.addEventListener('online', o)
      window.addEventListener('offline', o)
      return () => {
        window.removeEventListener('online', o)
        window.removeEventListener('offline', o)
      }
    },
    () => navigator.onLine,
  )
}

export function AvisoSinConexion() {
  const enLinea = useEnLinea()
  if (enLinea) return null
  return (
    <div className="flex items-center justify-center gap-2 bg-amber-100 px-4 py-1.5 text-sm text-amber-950" role="status">
      <WifiOff size={16} aria-hidden /> Sin conexión. Ves lo último que se cargó; para enviar reportes necesitas red.
    </div>
  )
}
