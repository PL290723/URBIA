// Sesión guardada en el navegador (token JWT + usuario).
//
// Se usa localStorage para que la app instalada en el celular no pida login cada vez
// que se abre. El token caduca según JWT_TTL de la API (por defecto 2 h): al vencer,
// la API responde 401 y aquí se borra la sesión.
import type { AuthResponse, Usuario } from '../api/tipos'

const CLAVE = 'urbia.sesion'

export interface Sesion {
  token: string
  expiraEn: string
  usuario: Usuario
}

type Oyente = () => void
const oyentes = new Set<Oyente>()
let cache: Sesion | null | undefined
let motivoCierre: 'expirada' | null = null

function leer(): Sesion | null {
  try {
    const crudo = localStorage.getItem(CLAVE)
    if (!crudo) return null
    const s = JSON.parse(crudo) as Sesion
    if (!s.token || !s.usuario) return null
    if (new Date(s.expiraEn).getTime() <= Date.now()) {
      localStorage.removeItem(CLAVE)
      motivoCierre = 'expirada'
      return null
    }
    return s
  } catch {
    return null
  }
}

function avisar() {
  for (const o of oyentes) o()
}

export function sesionGuardada(): Sesion | null {
  if (cache === undefined) cache = leer()
  // Revisa la caducidad cada vez que se pide (la app puede quedar abierta horas)
  if (cache && new Date(cache.expiraEn).getTime() <= Date.now()) {
    cache = null
    motivoCierre = 'expirada'
    try {
      localStorage.removeItem(CLAVE)
    } catch {
      /* almacenamiento no disponible */
    }
    queueMicrotask(avisar)
  }
  return cache
}

export function guardarSesion(r: AuthResponse) {
  cache = { token: r.token, expiraEn: r.expira_en, usuario: r.usuario }
  motivoCierre = null
  try {
    localStorage.setItem(CLAVE, JSON.stringify(cache))
  } catch {
    /* modo privado sin almacenamiento: la sesión vive solo en memoria */
  }
  avisar()
}

export function actualizarUsuario(u: Usuario) {
  if (!cache) return
  cache = { ...cache, usuario: u }
  try {
    localStorage.setItem(CLAVE, JSON.stringify(cache))
  } catch {
    /* ignorar */
  }
  avisar()
}

export function cerrarSesion() {
  cache = null
  try {
    localStorage.removeItem(CLAVE)
  } catch {
    /* ignorar */
  }
  avisar()
}

/** Lo llama el cliente HTTP cuando la API responde 401 con un token guardado. */
export function cerrarSesionPorToken() {
  motivoCierre = 'expirada'
  cerrarSesion()
}

/** Devuelve (y limpia) el motivo del último cierre automático. */
export function tomarMotivoCierre(): 'expirada' | null {
  const m = motivoCierre
  motivoCierre = null
  return m
}

export function suscribir(o: Oyente): () => void {
  oyentes.add(o)
  return () => oyentes.delete(o)
}

// Otra pestaña inició o cerró sesión
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (e) => {
    if (e.key === CLAVE) {
      cache = leer()
      avisar()
    }
  })
}
