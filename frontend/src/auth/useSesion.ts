import { useSyncExternalStore } from 'react'
import { sesionGuardada, suscribir, type Sesion } from './almacen'

export function useSesion(): Sesion | null {
  return useSyncExternalStore(suscribir, sesionGuardada, () => null)
}

export function useUsuario() {
  return useSesion()?.usuario ?? null
}

export function useEsAdmin(): boolean {
  return useUsuario()?.rol === 'administrador'
}
