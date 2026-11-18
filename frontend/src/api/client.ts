// Cliente HTTP de la API de URBIA.
//
// Todas las respuestas JSON llegan en el sobre {success, data} / {success, error}
// (ver openapi.yaml). Aquí se desenvuelven: las funciones devuelven `data` o lanzan ApiError.
import type { CodigoError, ErrorCampo, ErrorDetalle } from './tipos'
import { sesionGuardada, cerrarSesionPorToken } from '../auth/almacen'

/** '' = misma dirección del frontend (proxy de Vite). Ver .env.example. */
export const API_BASE = (import.meta.env.VITE_API_URL ?? '').replace(/\/$/, '')

export class ApiError extends Error {
  readonly status: number
  readonly code: CodigoError | 'SIN_CONEXION'
  readonly details: ErrorDetalle['details']
  readonly requestId: string | null
  readonly retryAfter: number | null

  constructor(
    status: number,
    code: ApiError['code'],
    message: string,
    details?: ErrorDetalle['details'],
    requestId: string | null = null,
    retryAfter: number | null = null,
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
    this.requestId = requestId
    this.retryAfter = retryAfter
  }

  /** Errores por campo (solo en VALIDACION_FALLIDA). */
  get erroresCampo(): ErrorCampo[] {
    return Array.isArray(this.details) ? this.details : []
  }

  /** Mensaje del primer error de un campo, si lo hay. */
  errorDe(campo: string): string | undefined {
    const e = this.erroresCampo.find((x) => x.campo === campo)
    return e ? (e.mensaje ?? `Revisa este campo (${e.regla})`) : undefined
  }
}

type Consulta = Record<string, string | number | boolean | undefined | null>

interface Opciones {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  query?: Consulta
  json?: unknown
  form?: FormData
  signal?: AbortSignal
  /** false = no mandar el token (login, registro, salud). */
  auth?: boolean
}

export function construirUrl(ruta: string, query?: Consulta): string {
  const url = `${API_BASE}${ruta}`
  if (!query) return url
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === '') continue
    params.set(k, String(v))
  }
  const qs = params.toString()
  return qs ? `${url}?${qs}` : url
}

function encabezados(auth: boolean, json: boolean): Headers {
  const h = new Headers({ Accept: 'application/json' })
  if (json) h.set('Content-Type', 'application/json')
  const sesion = sesionGuardada()
  if (auth && sesion) h.set('Authorization', `Bearer ${sesion.token}`)
  return h
}

/** Hace la petición y devuelve la respuesta cruda (lanza ApiError si no es 2xx). */
export async function peticionCruda(ruta: string, opciones: Opciones = {}): Promise<Response> {
  const { method = 'GET', query, json, form, signal, auth = true } = opciones
  let resp: Response
  try {
    resp = await fetch(construirUrl(ruta, query), {
      method,
      headers: encabezados(auth, json !== undefined),
      body: form ?? (json !== undefined ? JSON.stringify(json) : undefined),
      signal,
    })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    throw new ApiError(
      0,
      'SIN_CONEXION',
      navigator.onLine
        ? 'No se pudo contactar al servidor de URBIA. Revisa que la API esté encendida.'
        : 'Sin conexión a internet o a la red local.',
    )
  }

  if (resp.ok) return resp

  const requestId = resp.headers.get('X-Request-ID')
  const retry = Number(resp.headers.get('Retry-After'))
  let cuerpo: { error?: ErrorDetalle } | null = null
  try {
    cuerpo = await resp.json()
  } catch {
    /* respuesta sin JSON */
  }
  const err = cuerpo?.error
  const apiError = new ApiError(
    resp.status,
    err?.code ?? 'ERROR_INTERNO',
    err?.message ?? `Error ${resp.status} del servidor`,
    err?.details,
    requestId,
    Number.isFinite(retry) && retry > 0 ? retry : null,
  )

  // Token vencido o inválido: se cierra la sesión local para volver al login.
  if (resp.status === 401 && auth && sesionGuardada()) cerrarSesionPorToken()

  throw apiError
}

/** Petición JSON: devuelve el `data` del sobre de la API. */
export async function api<T>(ruta: string, opciones: Opciones = {}): Promise<T> {
  const resp = await peticionCruda(ruta, opciones)
  if (resp.status === 204) return undefined as T
  const cuerpo = (await resp.json()) as { success: boolean; data: T }
  return cuerpo.data
}

/** Descarga un archivo protegido (las fotos piden el token, no sirven en <img src>). */
export async function descargarBlob(ruta: string, signal?: AbortSignal): Promise<Blob> {
  const resp = await peticionCruda(ruta, { signal })
  return resp.blob()
}

/** Mensaje amigable para cualquier error. */
export function mensajeError(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === 'DEMASIADAS_SOLICITUDES' && e.retryAfter) {
      const min = Math.ceil(e.retryAfter / 60)
      return `${e.message} (${min} min aprox.)`
    }
    return e.message
  }
  if (e instanceof Error) return e.message
  return 'Ocurrió un error inesperado'
}
