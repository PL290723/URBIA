// Consultas y mutaciones de TanStack Query, una por endpoint de openapi.yaml.
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import { guardarSesion, actualizarUsuario } from '../auth/almacen'
import type {
  AuthResponse,
  CalcularPrioridadRequest,
  CalculoPrioridad,
  CambioEstado,
  Clasificacion,
  ConsultaPrioridad,
  Estado,
  Estadisticas,
  FiltrosCercanos,
  FiltrosEstadisticas,
  FiltrosReportes,
  HealthStatus,
  HistorialReporte,
  ListadoReportes,
  LoginRequest,
  NuevoReporte,
  RegistroRequest,
  Reporte,
  ReporteDetalle,
  ReportesCercanos,
  Usuario,
  Archivo,
} from './tipos'

const V1 = '/api/v1'

export const claves = {
  salud: ['salud'] as const,
  perfil: ['perfil'] as const,
  reportes: ['reportes'] as const,
  listado: (f: FiltrosReportes) => ['reportes', 'listado', f] as const,
  detalle: (id: number) => ['reportes', 'detalle', id] as const,
  historial: (id: number) => ['reportes', 'historial', id] as const,
  prioridad: (id: number) => ['reportes', 'prioridad', id] as const,
  cercanos: (id: number, f: FiltrosCercanos) => ['reportes', 'cercanos', id, f] as const,
  estadisticas: (f: FiltrosEstadisticas) => ['estadisticas', f] as const,
  usuarios: ['usuarios'] as const,
}

// ------------------------------------------------------------------ Salud
export function useSalud(habilitada = true) {
  return useQuery({
    queryKey: claves.salud,
    // 503 trae `data` con el detalle, pero aquí basta con saber que falló
    queryFn: ({ signal }) => api<HealthStatus>(`${V1}/health`, { auth: false, signal }),
    refetchInterval: 30_000,
    retry: false,
    enabled: habilitada,
  })
}

// ---------------------------------------------------------- Autenticación
export function useLogin() {
  return useMutation({
    mutationFn: (datos: LoginRequest) =>
      api<AuthResponse>(`${V1}/auth/login`, { method: 'POST', json: datos, auth: false }),
    onSuccess: guardarSesion,
  })
}

export function useRegistro() {
  return useMutation({
    mutationFn: (datos: RegistroRequest) =>
      api<AuthResponse>(`${V1}/auth/registro`, { method: 'POST', json: datos, auth: false }),
    onSuccess: guardarSesion,
  })
}

export function usePerfil() {
  return useQuery({
    queryKey: claves.perfil,
    queryFn: async ({ signal }) => {
      const u = await api<Usuario>(`${V1}/auth/perfil`, { signal })
      actualizarUsuario(u)
      return u
    },
  })
}

// --------------------------------------------------------------- Reportes
export function useReportes(filtros: FiltrosReportes, habilitada = true) {
  return useQuery({
    queryKey: claves.listado(filtros),
    queryFn: ({ signal }) => api<ListadoReportes>(`${V1}/reportes`, { query: filtros, signal }),
    placeholderData: keepPreviousData,
    enabled: habilitada,
  })
}

export function useReporte(id: number) {
  return useQuery({
    queryKey: claves.detalle(id),
    queryFn: ({ signal }) => api<ReporteDetalle>(`${V1}/reportes/${id}`, { signal }),
    enabled: Number.isInteger(id) && id > 0,
  })
}

export function useHistorial(id: number, habilitada = true) {
  return useQuery({
    queryKey: claves.historial(id),
    queryFn: ({ signal }) => api<HistorialReporte>(`${V1}/reportes/${id}/historial`, { signal }),
    enabled: habilitada && id > 0,
  })
}

export function useCrearReporte() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (r: NuevoReporte) => {
      const form = new FormData()
      const ext = r.foto.type === 'image/png' ? 'png' : r.foto.type === 'image/webp' ? 'webp' : 'jpg'
      form.append('foto', r.foto, `reporte.${ext}`)
      form.append('descripcion', r.descripcion.trim())
      form.append('latitud', r.latitud.toFixed(7))
      form.append('longitud', r.longitud.toFixed(7))
      form.append('origen_ubicacion', r.origen_ubicacion)
      if (r.origen_ubicacion === 'gps' && r.precision_m !== undefined) {
        form.append('precision_m', String(r.precision_m))
      }
      if (r.id_categoria) form.append('id_categoria', String(r.id_categoria))
      return api<Reporte>(`${V1}/reportes`, { method: 'POST', form })
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: claves.reportes }),
  })
}

export function useSubirFoto(idReporte: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (foto: Blob) => {
      const form = new FormData()
      form.append('foto', foto, 'foto.jpg')
      return api<Archivo>(`${V1}/reportes/${idReporte}/archivos`, { method: 'POST', form })
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: claves.detalle(idReporte) }),
  })
}

// ---------------------------------------------------------- Clasificación
export function useConfirmarCategoria(idReporte: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id_categoria: number) =>
      api<Clasificacion>(`${V1}/reportes/${idReporte}/categoria`, { method: 'PUT', json: { id_categoria } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: claves.reportes }),
  })
}

// --------------------------------------------------------- Administración
export function useCambiarEstado(idReporte: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (estado: Estado) =>
      api<CambioEstado>(`${V1}/reportes/${idReporte}/estado`, { method: 'PATCH', json: { estado } }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: claves.reportes })
      qc.invalidateQueries({ queryKey: ['estadisticas'] })
    },
  })
}

export function usePrioridad(idReporte: number, habilitada = true) {
  return useQuery({
    queryKey: claves.prioridad(idReporte),
    queryFn: ({ signal }) => api<ConsultaPrioridad>(`${V1}/reportes/${idReporte}/prioridad`, { signal }),
    enabled: habilitada && idReporte > 0,
  })
}

export function useCalcularPrioridad(idReporte: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (datos: CalcularPrioridadRequest | null) =>
      api<CalculoPrioridad>(`${V1}/reportes/${idReporte}/prioridad`, {
        method: 'POST',
        json: datos ?? undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: claves.prioridad(idReporte) })
      qc.invalidateQueries({ queryKey: claves.detalle(idReporte) })
      qc.invalidateQueries({ queryKey: ['reportes', 'listado'] })
    },
  })
}

export function useCercanos(idReporte: number, filtros: FiltrosCercanos, habilitada = true) {
  return useQuery({
    queryKey: claves.cercanos(idReporte, filtros),
    queryFn: ({ signal }) =>
      api<ReportesCercanos>(`${V1}/reportes/${idReporte}/cercanos`, { query: filtros, signal }),
    enabled: habilitada && idReporte > 0,
    placeholderData: keepPreviousData,
  })
}

export function useEstadisticas(filtros: FiltrosEstadisticas) {
  return useQuery({
    queryKey: claves.estadisticas(filtros),
    queryFn: ({ signal }) => api<Estadisticas>(`${V1}/estadisticas`, { query: filtros, signal }),
    placeholderData: keepPreviousData,
  })
}

export function useUsuarios() {
  return useQuery({
    queryKey: claves.usuarios,
    queryFn: ({ signal }) => api<Usuario[]>(`${V1}/admin/usuarios`, { signal }),
  })
}
