import { useEffect, useState } from 'react'
import { ImageOff } from 'lucide-react'
import { descargarBlob } from '../api/client'

type Resultado = { id: number; url: string | null; error: boolean }

/**
 * Las fotos de los reportes requieren el token (GET /api/v1/archivos/{id}) y la API
 * responde Cache-Control: no-store, así que no sirven en un <img src> normal.
 * Se descargan con fetch y se muestran como object URL.
 */
export function FotoProtegida({
  idArchivo,
  alt,
  className = '',
  enlace = true,
}: {
  idArchivo: number
  alt: string
  className?: string
  /** false cuando la foto ya está dentro de otro enlace (listas). */
  enlace?: boolean
}) {
  const [res, setRes] = useState<Resultado | null>(null)

  useEffect(() => {
    const ctrl = new AbortController()
    let objeto: string | null = null
    descargarBlob(`/api/v1/archivos/${idArchivo}`, ctrl.signal)
      .then((blob) => {
        objeto = URL.createObjectURL(blob)
        setRes({ id: idArchivo, url: objeto, error: false })
      })
      .catch((e) => {
        if (!(e instanceof DOMException && e.name === 'AbortError')) setRes({ id: idArchivo, url: null, error: true })
      })
    return () => {
      ctrl.abort()
      if (objeto) URL.revokeObjectURL(objeto)
    }
  }, [idArchivo])

  // Mientras llega la foto de este id (o si cambió el id) se muestra el esqueleto
  const actual = res?.id === idArchivo ? res : null

  if (actual?.error) {
    return (
      <div className={`flex flex-col items-center justify-center gap-1 bg-concreto text-xs text-gris ${className}`}>
        <ImageOff size={20} aria-hidden />
        {enlace && 'No se pudo cargar la foto'}
      </div>
    )
  }
  if (!actual?.url) return <div className={`animate-pulse bg-linea/60 ${className}`} aria-label="Cargando foto" />
  const img = <img src={actual.url} alt={alt} className={`object-cover ${className}`} />
  if (!enlace) return img
  return (
    <a href={actual.url} target="_blank" rel="noreferrer" title="Abrir la foto en tamaño completo">
      {img}
    </a>
  )
}
