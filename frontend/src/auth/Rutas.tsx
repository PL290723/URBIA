import { Navigate, Outlet, useLocation } from 'react-router'
import type { Rol } from '../api/tipos'
import { useSesion } from './useSesion'

/** Exige sesión; si se indica `rol`, además exige ese rol. */
export function RutaProtegida({ rol }: { rol?: Rol }) {
  const sesion = useSesion()
  const ubicacion = useLocation()

  if (!sesion) {
    return <Navigate to="/login" replace state={{ desde: ubicacion.pathname + ubicacion.search }} />
  }
  if (rol && sesion.usuario.rol !== rol) {
    return <Navigate to={sesion.usuario.rol === 'administrador' ? '/admin' : '/'} replace />
  }
  return <Outlet />
}

/** Solo para login/registro: si ya hay sesión, manda a la pantalla principal. */
export function RutaPublica() {
  const sesion = useSesion()
  if (sesion) return <Navigate to={sesion.usuario.rol === 'administrador' ? '/admin' : '/'} replace />
  return <Outlet />
}
