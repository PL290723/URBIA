const ZONA = 'America/Mexico_City'

const fFecha = new Intl.DateTimeFormat('es-MX', { dateStyle: 'medium', timeZone: ZONA })
const fFechaHora = new Intl.DateTimeFormat('es-MX', { dateStyle: 'medium', timeStyle: 'short', timeZone: ZONA })
const fRelativo = new Intl.RelativeTimeFormat('es-MX', { numeric: 'auto' })

export function fecha(iso: string): string {
  return fFecha.format(new Date(iso))
}

export function fechaHora(iso: string): string {
  return fFechaHora.format(new Date(iso))
}

/** "hace 3 horas", "ayer"… */
export function haceCuanto(iso: string): string {
  const seg = (new Date(iso).getTime() - Date.now()) / 1000
  const pasos: [Intl.RelativeTimeFormatUnit, number][] = [
    ['year', 31536000],
    ['month', 2592000],
    ['week', 604800],
    ['day', 86400],
    ['hour', 3600],
    ['minute', 60],
  ]
  for (const [unidad, s] of pasos) {
    if (Math.abs(seg) >= s) return fRelativo.format(Math.round(seg / s), unidad)
  }
  return 'hace un momento'
}

/** Día AAAA-MM-DD en hora de Celaya (formato de los filtros desde/hasta). */
export function diaLocal(d: Date): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: ZONA, year: 'numeric', month: '2-digit', day: '2-digit' }).format(d)
}

export function haceDias(n: number): string {
  return diaLocal(new Date(Date.now() - n * 86400000))
}

export function porcentaje(v: number | null | undefined, decimales = 0): string {
  if (v === null || v === undefined) return '—'
  return `${(v * 100).toFixed(decimales)} %`
}

export function bytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

export function metros(m: number): string {
  return m >= 1000 ? `${(m / 1000).toFixed(1)} km` : `${Math.round(m)} m`
}
