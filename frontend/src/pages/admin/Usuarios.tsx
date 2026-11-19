import { useMemo, useState } from 'react'
import { Search, SearchX, ShieldCheck } from 'lucide-react'
import { useUsuarios } from '../../api/consultas'
import { Cargando, Encabezado, MensajeError, Vacio } from '../../components/ui'
import { fecha } from '../../lib/formato'

export function Usuarios() {
  const { data, isPending, isError, error, refetch } = useUsuarios()
  const [busqueda, setBusqueda] = useState('')

  const filtrados = useMemo(() => {
    const q = busqueda.trim().toLowerCase()
    if (!data) return []
    return q ? data.filter((u) => u.nombre.toLowerCase().includes(q) || u.correo.toLowerCase().includes(q)) : data
  }, [data, busqueda])

  return (
    <div>
      <Encabezado
        titulo="Usuarios"
        subtitulo={
          data
            ? `${data.length} cuentas, de ellas ${data.filter((u) => u.rol === 'administrador').length} con acceso de administración`
            : undefined
        }
      />
      <label className="relative mb-5 block max-w-sm">
        <span className="sr-only">Buscar</span>
        <Search size={18} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-gris-claro" aria-hidden />
        <input
          type="search"
          className="campo pl-10"
          placeholder="Buscar por nombre o correo"
          value={busqueda}
          onChange={(e) => setBusqueda(e.target.value)}
        />
      </label>
      {isPending ? (
        <Cargando />
      ) : isError ? (
        <MensajeError error={error} reintentar={refetch} />
      ) : filtrados.length === 0 ? (
        <Vacio Icono={SearchX} titulo="Nadie coincide con la búsqueda" />
      ) : (
        <div className="hoja overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="border-b border-linea text-left text-gris">
              <tr>
                <th className="px-4 py-3 font-medium">Nombre</th>
                <th className="px-4 py-3 font-medium">Correo</th>
                <th className="px-4 py-3 font-medium">Acceso</th>
                <th className="hidden px-4 py-3 font-medium sm:table-cell">Alta</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-linea">
              {filtrados.map((u) => (
                <tr key={u.id_usuario}>
                  <td className="px-4 py-3 font-medium">{u.nombre}</td>
                  <td className="max-w-48 truncate px-4 py-3 text-gris">{u.correo}</td>
                  <td className="px-4 py-3">
                    {u.rol === 'administrador' ? (
                      <span className="inline-flex items-center gap-1.5 font-medium text-urbia-800">
                        <ShieldCheck size={16} aria-hidden /> Administración
                      </span>
                    ) : (
                      <span className="text-gris">Ciudadano</span>
                    )}
                  </td>
                  <td className="hidden px-4 py-3 text-gris sm:table-cell">{fecha(u.fecha_registro)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
