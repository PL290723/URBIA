// Preparación de la foto antes de subirla.
//
// Las fotos de celular pesan 3–8 MB y miden 4000 px o más. La API acepta hasta 10 MB y
// 8000 px por lado, pero en la red de la feria conviene subir menos: se reduce a 2560 px
// por el lado mayor y se recomprime a JPEG. El backend vuelve a validar y quita el EXIF.

const LADO_MAX = 2560
const LADO_MIN = 200
export const TAMANO_MAX = 10 * 1024 * 1024
const TIPOS = ['image/jpeg', 'image/png', 'image/webp']

export interface FotoPreparada {
  blob: Blob
  ancho: number
  alto: number
  reducida: boolean
}

export class ErrorFoto extends Error {}

export async function prepararFoto(archivo: File): Promise<FotoPreparada> {
  if (archivo.type && !TIPOS.includes(archivo.type) && !archivo.type.startsWith('image/')) {
    throw new ErrorFoto('El archivo no es una imagen. Usa una foto JPEG, PNG o WebP.')
  }

  let bitmap: ImageBitmap
  try {
    // imageOrientation: respeta la rotación del EXIF de las fotos del celular
    bitmap = await createImageBitmap(archivo, { imageOrientation: 'from-image' })
  } catch {
    // HEIC de iPhone u otro formato que el navegador no sabe abrir
    if (TIPOS.includes(archivo.type) && archivo.size <= TAMANO_MAX) {
      return { blob: archivo, ancho: 0, alto: 0, reducida: false }
    }
    throw new ErrorFoto('No se pudo leer la imagen. Toma la foto de nuevo o usa JPEG/PNG/WebP.')
  }

  const { width, height } = bitmap
  if (width < LADO_MIN || height < LADO_MIN) {
    bitmap.close()
    throw new ErrorFoto(`La foto es muy pequeña (mínimo ${LADO_MIN} px por lado).`)
  }

  const escala = Math.min(1, LADO_MAX / Math.max(width, height))
  const necesitaReducir = escala < 1 || archivo.size > 2.5 * 1024 * 1024 || !TIPOS.includes(archivo.type)
  if (!necesitaReducir) {
    bitmap.close()
    return { blob: archivo, ancho: width, alto: height, reducida: false }
  }

  const ancho = Math.round(width * escala)
  const alto = Math.round(height * escala)
  const canvas = document.createElement('canvas')
  canvas.width = ancho
  canvas.height = alto
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    bitmap.close()
    return { blob: archivo, ancho: width, alto: height, reducida: false }
  }
  ctx.drawImage(bitmap, 0, 0, ancho, alto)
  bitmap.close()

  const blob = await new Promise<Blob | null>((ok) => canvas.toBlob(ok, 'image/jpeg', 0.85))
  if (!blob) return { blob: archivo, ancho: width, alto: height, reducida: false }
  if (blob.size > TAMANO_MAX) throw new ErrorFoto('La foto supera 10 MB incluso después de reducirla.')
  return { blob, ancho, alto, reducida: true }
}
