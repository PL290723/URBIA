import { useNavigate } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { LogOut } from 'lucide-react'
import { usePerfil } from '../api/consultas'
import { cerrarSesion } from '../auth/almacen'
import { useSesion } from '../auth/useSesion'
import { Dato, Encabezado, Logo, MensajeError } from '../components/ui'
import { BotonInstalar } from '../pwa/pwa'
import { esAppInstalada } from '../pwa/instalacion'
import { fecha, fechaHora } from '../lib/formato'

export function Perfil() {
  const sesion = useSesion()
  const { data, isError, error } = usePerfil()
  const qc = useQueryClient()
  const navegar = useNavigate()
  const u = data ?? sesion?.usuario

  const salir = () => {
    cerrarSesion()
    qc.clear() // no dejar datos del usuario anterior en memoria
    navegar('/login', { replace: true })
  }

  return (
    <div className="mx-auto max-w-lg">
      <Encabezado titulo="Perfil" />
      {isError && <MensajeError error={error} />}
      {u && (
        <section className="hoja p-5">
          <div className="flex items-center gap-4">
            <span className="titulo flex size-14 items-center justify-center rounded-full bg-urbia-800 text-2xl text-white" aria-hidden>
              {u.nombre.charAt(0).toUpperCase()}
            </span>
            <div className="min-w-0">
              <p className="text-lg font-semibold">{u.nombre}</p>
              <p className="truncate text-gris">{u.correo}</p>
            </div>
          </div>
          <dl className="mt-5 grid grid-cols-2 gap-4 border-t border-linea pt-5 text-sm">
            <Dato etiqueta="Tipo de cuenta">{u.rol === 'administrador' ? 'Administración' : 'Ciudadano'}</Dato>
            <Dato etiqueta="Con URBIA desde">{fecha(u.fecha_registro)}</Dato>
            {sesion && (
              <div className="col-span-2">
                <Dato etiqueta="Tu sesión termina">{fechaHora(sesion.expiraEn)}</Dato>
              </div>
            )}
          </dl>
        </section>
      )}

      {!esAppInstalada() && (
        <section className="mt-4">
          <BotonInstalar className="w-full" />
        </section>
      )}

      <button type="button" className="btn-peligro mt-4 w-full" onClick={salir}>
        <LogOut size={18} aria-hidden /> Cerrar sesión
      </button>

      <footer className="mt-12 flex flex-col items-center text-center">
        <Logo variante="completo" className="w-32" />
        <p className="mt-3 text-xs text-gris">Feria de Proyectos TecNM Celaya 2026, versión {__VERSION__}</p>
      </footer>
    </div>
  )
}
