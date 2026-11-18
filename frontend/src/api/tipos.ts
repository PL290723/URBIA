// Alias legibles de los tipos generados desde backend-go/internal/apidocs/openapi.yaml.
// No editar schema.d.ts a mano: se regenera con `npm run gen:api`.
import type { components, operations } from './schema'

type S = components['schemas']

export type Estado = S['Estado']
export type Rol = S['Rol']
export type NivelPrioridad = S['NivelPrioridad']
export type Categoria = S['Categoria']
export type OrigenCategoria = S['OrigenCategoria']
export type EstadoSugerencia = S['EstadoSugerencia']

export type Usuario = S['Usuario']
export type AuthResponse = S['AuthResponse']
export type RegistroRequest = S['RegistroRequest']
export type LoginRequest = S['LoginRequest']

export type HealthStatus = S['HealthStatus']

export type Reporte = S['Reporte']
export type ReporteResumen = S['ReporteResumen']
export type ReporteDetalle = S['ReporteDetalle']
export type ListadoReportes = S['ListadoReportes']
export type Paginacion = S['Paginacion']
export type HistorialReporte = S['HistorialReporte']
export type HistorialEstado = S['HistorialEstado']
export type Archivo = S['Archivo']
export type DatosAdministrativos = S['DatosAdministrativos']

export type Clasificacion = S['Clasificacion']
export type CambioEstado = S['CambioEstado']
export type CalcularPrioridadRequest = S['CalcularPrioridadRequest']
export type CalculoPrioridad = S['CalculoPrioridad']
export type ConsultaPrioridad = S['ConsultaPrioridad']
export type FactorPrioridad = S['FactorPrioridad']
export type ReportesCercanos = S['ReportesCercanos']
export type Coincidencia = S['Coincidencia']
export type Estadisticas = S['Estadisticas']

export type ErrorDetalle = S['ErrorDetalle']
export type CodigoError = S['CodigoError']
export type ErrorCampo = S['ErrorCampo']

/** Parámetros de GET /api/v1/reportes (salen directo del contrato). */
export type FiltrosReportes = NonNullable<operations['listarReportes']['parameters']['query']>
/** Parámetros de GET /api/v1/estadisticas. */
export type FiltrosEstadisticas = NonNullable<operations['obtenerEstadisticas']['parameters']['query']>
/** Parámetros de GET /api/v1/reportes/{id}/cercanos. */
export type FiltrosCercanos = NonNullable<operations['buscarCercanos']['parameters']['query']>

/** Campos del formulario multipart de POST /api/v1/reportes (la foto va aparte). */
export interface NuevoReporte {
  foto: Blob
  descripcion: string
  latitud: number
  longitud: number
  origen_ubicacion: 'gps' | 'manual'
  precision_m?: number
  id_categoria?: number
}
