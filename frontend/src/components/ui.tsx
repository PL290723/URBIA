// Piezas pequeñas de interfaz reutilizadas en todas las pantallas.
import { createElement, type ReactNode } from 'react'
import { ChevronLeft, ChevronRight, CircleAlert, Info, CircleCheck, TriangleAlert, type LucideIcon } from 'lucide-react'
import type { Categoria, Estado, NivelPrioridad } from '../api/tipos'
import { ApiError, mensajeError } from '../api/client'
import { ESTADO_INFO, PRIORIDAD_INFO, iconoCategoria } from '../lib/catalogos'

export function Logo({ variante = 'marca', className = '' }: { variante?: 'marca' | 'completo'; className?: string }) {
  return (
    <img
      src={variante === 'marca' ? '/logo-marca.png' : '/logo-urbia.png'}
      alt={variante === 'marca' ? 'URBIA' : 'URBIA, detección inteligente de problemas urbanos'}
      className={className}
      draggable={false}
    />
  )
}

/** Ícono de la categoría dentro de un cuadro con el tono de la marca. */
export function IconoCategoria({
  categoria,
  tamano = 'md',
}: {
  categoria: Pick<Categoria, 'nombre'> | null | undefined
  tamano?: 'sm' | 'md' | 'lg'
}) {
  const caja = { sm: 'size-8 rounded-lg', md: 'size-10 rounded-[10px]', lg: 'size-14 rounded-xl' }[tamano]
  const icono = { sm: 16, md: 20, lg: 28 }[tamano]
  return (
    <span className={`inline-flex shrink-0 items-center justify-center bg-urbia-100 text-urbia-800 ${caja}`} aria-hidden>
      {createElement(iconoCategoria(categoria), { size: icono, strokeWidth: 1.75 })}
    </span>
  )
}

export function InsigniaEstado({ estado, className = '' }: { estado: Estado; className?: string }) {
  const info = ESTADO_INFO[estado]
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold whitespace-nowrap ${info.clases} ${className}`}>
      <span className="size-2 rounded-full" style={{ backgroundColor: info.color }} aria-hidden />
      {info.etiqueta}
    </span>
  )
}

/** Prioridad como barras de señal (1 a 3), con el nombre del nivel al lado. */
export function InsigniaPrioridad({ nivel, conTexto = true }: { nivel: NivelPrioridad | null | undefined; conTexto?: boolean }) {
  if (!nivel) return <span className="text-xs text-gris-claro">Sin nivel</span>
  const info = PRIORIDAD_INFO[nivel]
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-semibold ${info.clases}`} title={`Prioridad ${info.etiqueta.toLowerCase()}`}>
      <span className="flex items-end gap-[2px]" aria-hidden>
        {[1, 2, 3].map((b) => (
          <span
            key={b}
            className={`w-[4px] rounded-[1px] ${b <= info.barras ? 'bg-current' : 'bg-linea'}`}
            style={{ height: 4 + b * 3 }}
          />
        ))}
      </span>
      {conTexto && info.etiqueta}
    </span>
  )
}

export function Spinner({ className = 'size-4' }: { className?: string }) {
  return <span className={`inline-block animate-spin rounded-full border-2 border-current border-t-transparent ${className}`} aria-hidden />
}

export function Cargando({ texto = 'Cargando', className = '' }: { texto?: string; className?: string }) {
  return (
    <div className={`flex items-center justify-center gap-3 py-12 text-gris ${className}`} role="status">
      <Spinner className="size-5 text-urbia-600" />
      <span className="text-sm">{texto}</span>
    </div>
  )
}

export function MensajeError({ error, reintentar }: { error: unknown; reintentar?: () => void }) {
  const id = error instanceof ApiError ? error.requestId : null
  return (
    <div className="flex gap-3 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-900" role="alert">
      <CircleAlert size={20} className="mt-px shrink-0 text-red-600" aria-hidden />
      <div>
        <p className="font-medium">{mensajeError(error)}</p>
        {id && <p className="mt-1 text-xs text-red-700">Folio para soporte: {id}</p>}
        {reintentar && (
          <button type="button" onClick={reintentar} className="mt-2 font-semibold text-red-800 underline underline-offset-2">
            Reintentar
          </button>
        )}
      </div>
    </div>
  )
}

const AVISOS: Record<'info' | 'exito' | 'alerta', { clases: string; Icono: LucideIcon }> = {
  info: { clases: 'border-urbia-100 bg-urbia-50 text-urbia-900', Icono: Info },
  exito: { clases: 'border-green-200 bg-green-50 text-green-900', Icono: CircleCheck },
  alerta: { clases: 'border-amber-200 bg-amber-50 text-amber-950', Icono: TriangleAlert },
}

export function Aviso({ tipo = 'info', children }: { tipo?: keyof typeof AVISOS; children: ReactNode }) {
  const { clases, Icono } = AVISOS[tipo]
  return (
    <div className={`flex gap-2.5 rounded-xl border p-3 text-sm ${clases}`}>
      <Icono size={18} className="mt-px shrink-0 opacity-80" aria-hidden />
      <div>{children}</div>
    </div>
  )
}

export function Vacio({ Icono, titulo, children }: { Icono: LucideIcon; titulo: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-14 text-center">
      <span className="flex size-14 items-center justify-center rounded-full bg-white text-urbia-700 ring-1 ring-linea">
        <Icono size={26} strokeWidth={1.5} aria-hidden />
      </span>
      <p className="font-semibold">{titulo}</p>
      {children && <div className="max-w-xs text-sm text-gris">{children}</div>}
    </div>
  )
}

export function Encabezado({ titulo, subtitulo, acciones }: { titulo: string; subtitulo?: ReactNode; acciones?: ReactNode }) {
  return (
    <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="titulo text-2xl sm:text-3xl">{titulo}</h1>
        {subtitulo && <div className="mt-1 text-sm text-gris">{subtitulo}</div>}
      </div>
      {acciones && <div className="flex flex-wrap gap-2">{acciones}</div>}
    </div>
  )
}

export function Paginador({
  pagina,
  totalPaginas,
  total,
  onCambiar,
}: {
  pagina: number
  totalPaginas: number
  total: number
  onCambiar: (p: number) => void
}) {
  if (totalPaginas <= 1) {
    return <p className="py-4 text-center text-xs text-gris">{total === 1 ? '1 reporte' : `${total} reportes`}</p>
  }
  return (
    <nav className="flex items-center justify-between gap-2 py-4" aria-label="Paginación">
      <button type="button" className="btn-secundario px-3" disabled={pagina <= 1} onClick={() => onCambiar(pagina - 1)}>
        <ChevronLeft size={18} aria-hidden /> Anterior
      </button>
      <span className="text-sm text-gris tabular-nums">
        {pagina} de {totalPaginas}
      </span>
      <button type="button" className="btn-secundario px-3" disabled={pagina >= totalPaginas} onClick={() => onCambiar(pagina + 1)}>
        Siguiente <ChevronRight size={18} aria-hidden />
      </button>
    </nav>
  )
}

export function Dato({ etiqueta, children }: { etiqueta: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-sm text-gris">{etiqueta}</dt>
      <dd className="mt-0.5 font-medium">{children}</dd>
    </div>
  )
}
