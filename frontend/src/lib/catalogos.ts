// Catálogos y textos para mostrar en pantalla.
import {
  CircleHelp,
  Construction,
  Droplets,
  Footprints,
  Lightbulb,
  Signpost,
  SprayCan,
  Trash2,
  TreeDeciduous,
  type LucideIcon,
} from 'lucide-react'
import {
  CircleHelp as nCircleHelp,
  Construction as nConstruction,
  Droplets as nDroplets,
  Footprints as nFootprints,
  Lightbulb as nLightbulb,
  Signpost as nSignpost,
  SprayCan as nSprayCan,
  Trash2 as nTrash2,
  TreeDeciduous as nTreeDeciduous,
  type IconNode,
} from 'lucide'
import type { Estado, NivelPrioridad, OrigenCategoria, Categoria } from '../api/tipos'

/**
 * Categorías de database/migrations/00002_datos_semilla.sql (mismo orden = mismo id).
 * La API todavía no expone GET /categorias; si se agrega una categoría nueva hay que
 * sumarla aquí también. `Icono` es para React y `nodo` para los marcadores de Leaflet.
 */
interface InfoCategoria extends Categoria {
  etiqueta: string
  descripcion: string
  Icono: LucideIcon
  nodo: IconNode
}

export const CATEGORIAS: InfoCategoria[] = [
  { id_categoria: 1, nombre: 'bache', etiqueta: 'Bache', descripcion: 'Hundimiento o daño en el pavimento', Icono: Construction, nodo: nConstruction },
  { id_categoria: 2, nombre: 'alumbrado_publico', etiqueta: 'Alumbrado público', descripcion: 'Luminaria apagada, intermitente o dañada', Icono: Lightbulb, nodo: nLightbulb },
  { id_categoria: 3, nombre: 'basura', etiqueta: 'Basura', descripcion: 'Basura acumulada o tiradero en la calle', Icono: Trash2, nodo: nTrash2 },
  { id_categoria: 4, nombre: 'fuga_agua', etiqueta: 'Fuga de agua', descripcion: 'Fuga de agua potable o drenaje', Icono: Droplets, nodo: nDroplets },
  { id_categoria: 5, nombre: 'banqueta_danada', etiqueta: 'Banqueta dañada', descripcion: 'Banqueta rota, levantada o con obstáculos', Icono: Footprints, nodo: nFootprints },
  { id_categoria: 6, nombre: 'senalizacion', etiqueta: 'Señalización', descripcion: 'Señal de tránsito dañada, caída o faltante', Icono: Signpost, nodo: nSignpost },
  { id_categoria: 7, nombre: 'grafiti', etiqueta: 'Grafiti', descripcion: 'Pintas en muros, mobiliario o señales', Icono: SprayCan, nodo: nSprayCan },
  { id_categoria: 8, nombre: 'arbol_caido', etiqueta: 'Árbol caído', descripcion: 'Árbol o rama que obstruye el paso', Icono: TreeDeciduous, nodo: nTreeDeciduous },
]

/** Texto legible de una categoría (usa el catálogo y, si no está, adapta el nombre). */
export function nombreCategoria(c: { nombre: string } | null | undefined): string {
  if (!c) return 'Sin categoría'
  const conocida = CATEGORIAS.find((x) => x.nombre === c.nombre)
  if (conocida) return conocida.etiqueta
  const t = c.nombre.replace(/_/g, ' ')
  return t.charAt(0).toUpperCase() + t.slice(1)
}

export function iconoCategoria(c: { nombre: string } | null | undefined): LucideIcon {
  return CATEGORIAS.find((x) => x.nombre === c?.nombre)?.Icono ?? CircleHelp
}

export function nodoCategoria(c: { nombre: string } | null | undefined): IconNode {
  return CATEGORIAS.find((x) => x.nombre === c?.nombre)?.nodo ?? nCircleHelp
}

export const ESTADOS: Estado[] = ['pendiente', 'en_revision', 'en_proceso', 'resuelto']

export const ESTADO_INFO: Record<Estado, { etiqueta: string; color: string; clases: string }> = {
  pendiente: { etiqueta: 'Pendiente', color: '#d97706', clases: 'bg-amber-50 text-amber-900' },
  en_revision: { etiqueta: 'En revisión', color: '#2563eb', clases: 'bg-blue-50 text-blue-900' },
  en_proceso: { etiqueta: 'En proceso', color: '#db2777', clases: 'bg-pink-50 text-pink-900' },
  resuelto: { etiqueta: 'Resuelto', color: '#16a34a', clases: 'bg-green-50 text-green-900' },
}

/** Siguiente estado permitido (pendiente → en_revision → en_proceso → resuelto). */
export function siguienteEstado(e: Estado): Estado | null {
  const i = ESTADOS.indexOf(e)
  return i >= 0 && i < ESTADOS.length - 1 ? ESTADOS[i + 1] : null
}

export const PRIORIDAD_INFO: Record<NivelPrioridad, { etiqueta: string; clases: string; barras: number }> = {
  baja: { etiqueta: 'Baja', clases: 'text-gris', barras: 1 },
  media: { etiqueta: 'Media', clases: 'text-orange-700', barras: 2 },
  alta: { etiqueta: 'Alta', clases: 'text-red-700', barras: 3 },
}

export const ORIGEN_CATEGORIA: Record<OrigenCategoria, string> = {
  sugerida: 'Aceptó la sugerencia de la IA',
  corregida: 'Corrigió la sugerencia de la IA',
  manual: 'Elegida por el ciudadano',
}

export const FACTOR_PRIORIDAD: Record<string, string> = {
  gravedad_observable: 'Gravedad observable',
  afectacion_estimada: 'Afectación estimada',
  recurrencia_verificada: 'Recurrencia verificada',
  antiguedad: 'Antigüedad',
}
