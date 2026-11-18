// Crear un reporte: foto + ubicación + descripción (HU de creación de reportes).
import { useCallback, useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { Camera, Check, Image as ImageIcon, LocateFixed, MapPin, Send } from 'lucide-react'
import { useCrearReporte } from '../api/consultas'
import { ApiError } from '../api/client'
import { SelectorUbicacion } from '../components/mapa'
import { Aviso, Encabezado, MensajeError, Spinner } from '../components/ui'
import { CATEGORIAS } from '../lib/catalogos'
import { bytes } from '../lib/formato'
import { PRECISION_MAXIMA_M, gpsDisponible, useGps } from '../lib/geo'
import { ErrorFoto, prepararFoto, type FotoPreparada } from '../lib/imagen'

type Origen = 'gps' | 'manual'

/** Los tres pasos sí son una secuencia: número a la izquierda unido por una línea. */
function Paso({ n, titulo, listo, ultimo = false, children }: { n: number; titulo: string; listo: boolean; ultimo?: boolean; children: React.ReactNode }) {
  return (
    <section className="grid grid-cols-[2.25rem_1fr] gap-x-3 sm:gap-x-4">
      <div className="flex flex-col items-center">
        <span
          className={`titulo flex size-9 shrink-0 items-center justify-center rounded-full text-lg ${
            listo ? 'bg-urbia-800 text-white' : 'border-2 border-urbia-800 bg-white text-urbia-800'
          }`}
        >
          {listo ? <Check size={18} strokeWidth={3} aria-label="Listo" /> : n}
        </span>
        {!ultimo && <span className="my-1 w-0.5 flex-1 bg-linea" aria-hidden />}
      </div>
      <div className={ultimo ? '' : 'pb-8'}>
        <h2 className="titulo mt-1 mb-3 text-xl">{titulo}</h2>
        {children}
      </div>
    </section>
  )
}

export function NuevoReporte() {
  const navegar = useNavigate()
  const crear = useCrearReporte()
  const gps = useGps()
  const detenerGps = gps.detener

  // ---- Foto
  const [foto, setFoto] = useState<FotoPreparada | null>(null)
  const [vistaPrevia, setVistaPrevia] = useState<string | null>(null)
  const [errorFoto, setErrorFoto] = useState<string | null>(null)
  const [procesando, setProcesando] = useState(false)
  const camara = useRef<HTMLInputElement>(null)
  const galeria = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!foto) return
    const url = URL.createObjectURL(foto.blob)
    setVistaPrevia(url)
    return () => URL.revokeObjectURL(url)
  }, [foto])

  const elegirFoto = async (e: ChangeEvent<HTMLInputElement>) => {
    const archivo = e.target.files?.[0]
    e.target.value = ''
    if (!archivo) return
    setErrorFoto(null)
    setProcesando(true)
    try {
      setFoto(await prepararFoto(archivo))
    } catch (err) {
      setErrorFoto(err instanceof ErrorFoto ? err.message : 'No se pudo procesar la foto.')
    } finally {
      setProcesando(false)
    }
  }

  // ---- Ubicación
  const [punto, setPunto] = useState<[number, number] | null>(null)
  const [origen, setOrigen] = useState<Origen>('gps')
  const [precision, setPrecision] = useState<number | undefined>()

  // Al entrar se pide el GPS automáticamente
  useEffect(() => {
    if (gpsDisponible) gps.buscar()
    else setOrigen('manual')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Cada lectura nueva del GPS mueve el pin, salvo que el usuario ya lo haya movido a mano
  useEffect(() => {
    if (gps.estado.tipo === 'listo' && origen === 'gps') {
      setPunto([gps.estado.lat, gps.estado.lon])
      setPrecision(gps.estado.precision)
    }
  }, [gps.estado, origen])

  const moverPin = useCallback(
    (lat: number, lon: number) => {
      detenerGps()
      setPunto([lat, lon])
      setOrigen('manual')
      setPrecision(undefined)
    },
    [detenerGps],
  )

  const usarGps = () => {
    setOrigen('gps')
    gps.buscar()
  }

  const gpsImpreciso = origen === 'gps' && precision !== undefined && precision > PRECISION_MAXIMA_M

  // ---- Descripción y categoría
  const [descripcion, setDescripcion] = useState('')
  const [categoria, setCategoria] = useState<number | undefined>()
  const largo = descripcion.trim().length

  const listoFoto = !!foto
  const listoUbicacion = !!punto && !gpsImpreciso && (origen === 'manual' || precision !== undefined)
  const listoDescripcion = largo >= 10 && largo <= 1000
  const puedeEnviar = listoFoto && listoUbicacion && listoDescripcion && !crear.isPending

  const err = crear.error instanceof ApiError ? crear.error : null

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    if (!foto || !punto || !puedeEnviar) return
    crear.mutate(
      {
        foto: foto.blob,
        descripcion,
        latitud: punto[0],
        longitud: punto[1],
        origen_ubicacion: origen,
        precision_m: origen === 'gps' ? precision : undefined,
        id_categoria: categoria,
      },
      {
        onSuccess: (r) => navegar(`/reportes/${r.id_reporte}/categoria`, { replace: true, state: { reporte: r } }),
      },
    )
  }

  const errorFotoTexto =
    errorFoto ??
    err?.errorDe('foto') ??
    (err && ['FORMATO_NO_PERMITIDO', 'IMAGEN_INVALIDA', 'ARCHIVO_DEMASIADO_GRANDE'].includes(err.code) ? err.message : undefined)
  const errorUbicacion = err?.errorDe('latitud') ?? err?.errorDe('longitud') ?? err?.errorDe('precision_m')

  return (
    <form onSubmit={enviar} className="mx-auto max-w-2xl">
      <Encabezado titulo="Nuevo reporte" subtitulo="Tres pasos: foto, lugar y una descripción breve." />

      <Paso n={1} titulo="Toma una foto del problema" listo={listoFoto}>
        <input ref={camara} type="file" accept="image/*" capture="environment" hidden onChange={elegirFoto} />
        <input ref={galeria} type="file" accept="image/jpeg,image/png,image/webp,image/*" hidden onChange={elegirFoto} />
        {vistaPrevia ? (
          <figure>
            <img src={vistaPrevia} alt="Foto que se va a enviar" className="max-h-80 w-full rounded-2xl border border-linea bg-white object-contain" />
            <figcaption className="mt-2 text-sm text-gris">
              {bytes(foto!.blob.size)}
              {foto!.ancho > 0 && `, ${foto!.ancho} × ${foto!.alto} px`}
              {foto!.reducida && '. Se redujo para enviarla más rápido.'}
            </figcaption>
          </figure>
        ) : (
          <button
            type="button"
            onClick={() => camara.current?.click()}
            className="flex h-44 w-full flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed border-urbia-300 bg-white text-urbia-800 hover:border-urbia-500"
          >
            {procesando ? <Spinner className="size-7" /> : <Camera size={36} strokeWidth={1.5} aria-hidden />}
            <span className="font-semibold">{procesando ? 'Preparando la foto' : 'Abrir la cámara'}</span>
          </button>
        )}
        <div className="mt-3 flex flex-wrap gap-2">
          {vistaPrevia && (
            <button type="button" className="btn-secundario" onClick={() => camara.current?.click()}>
              <Camera size={18} aria-hidden /> Tomar otra
            </button>
          )}
          <button type="button" className="btn-secundario" onClick={() => galeria.current?.click()}>
            <ImageIcon size={18} aria-hidden /> Elegir de la galería
          </button>
        </div>
        {errorFotoTexto && <p className="mt-2 text-sm text-red-700">{errorFotoTexto}</p>}
      </Paso>

      <Paso n={2} titulo="Marca dónde está" listo={listoUbicacion}>
        <div className="mb-3 flex min-h-6 flex-wrap items-center gap-x-4 gap-y-1 text-sm">
          {origen === 'gps' && gps.estado.tipo === 'buscando' && (
            <span className="inline-flex items-center gap-2 text-gris">
              <Spinner className="size-4 text-urbia-600" /> Buscando tu ubicación
            </span>
          )}
          {origen === 'gps' && gps.estado.tipo === 'listo' && !gpsImpreciso && (
            <span className="inline-flex items-center gap-1.5 text-urbia-800">
              <LocateFixed size={16} aria-hidden /> Tu ubicación, con precisión de {Math.round(gps.estado.precision)} m
            </span>
          )}
          {origen === 'manual' && punto && (
            <span className="inline-flex items-center gap-1.5 text-urbia-800">
              <MapPin size={16} aria-hidden /> Punto marcado en el mapa
            </span>
          )}
          {gpsDisponible && origen === 'manual' && (
            <button type="button" onClick={usarGps} className="btn-texto">
              Usar mi ubicación
            </button>
          )}
        </div>
        {gps.estado.tipo === 'error' && origen === 'gps' && (
          <div className="mb-3">
            <Aviso tipo="alerta">{gps.estado.mensaje}</Aviso>
          </div>
        )}
        {gpsImpreciso && (
          <div className="mb-3">
            <Aviso tipo="alerta">
              Tu ubicación tiene un margen de {Math.round(precision!)} m. Toca el mapa en el lugar exacto del problema.
            </Aviso>
          </div>
        )}
        <div className="overflow-hidden rounded-2xl border border-linea">
          <SelectorUbicacion punto={punto} precision={origen === 'gps' ? precision : undefined} onCambio={moverPin} />
        </div>
        <p className="mt-2 text-sm text-gris">Si el punto no es exacto, toca el mapa o arrastra el marcador amarillo.</p>
        {errorUbicacion && <p className="mt-2 text-sm text-red-700">{errorUbicacion}</p>}
      </Paso>

      <Paso n={3} titulo="Cuéntanos qué pasa" listo={listoDescripcion} ultimo>
        <label className="block">
          <span className="sr-only">Descripción</span>
          <textarea
            className="campo min-h-28 resize-y"
            maxLength={1000}
            placeholder="Por ejemplo: bache profundo frente a la escuela, ocupa medio carril."
            value={descripcion}
            onChange={(e) => setDescripcion(e.target.value)}
          />
        </label>
        <p className={`mt-1.5 text-sm tabular-nums ${largo > 0 && largo < 10 ? 'text-red-700' : 'text-gris'}`}>
          {largo < 10 ? `Escribe al menos ${10 - largo} caracteres más` : `${largo} de 1000 caracteres`}
        </p>
        {err?.errorDe('descripcion') && <p className="text-sm text-red-700">{err.errorDe('descripcion')}</p>}

        <fieldset className="mt-6">
          <legend className="font-medium">Tipo de problema</legend>
          <p className="mb-3 text-sm text-gris">Opcional. Si no eliges, URBIA lo sugiere a partir de la foto.</p>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {CATEGORIAS.map((c) => {
              const marcada = categoria === c.id_categoria
              return (
                <button
                  key={c.id_categoria}
                  type="button"
                  aria-pressed={marcada}
                  onClick={() => setCategoria((x) => (x === c.id_categoria ? undefined : c.id_categoria))}
                  className={`flex items-center gap-2.5 rounded-[10px] border px-3 py-2.5 text-left text-sm font-medium ${
                    marcada ? 'border-urbia-800 bg-urbia-800 text-white' : 'border-linea bg-white hover:border-urbia-300'
                  }`}
                >
                  <c.Icono size={18} strokeWidth={1.75} className="shrink-0" aria-hidden />
                  {c.etiqueta}
                </button>
              )
            })}
          </div>
        </fieldset>
      </Paso>

      <div className="mt-8 space-y-3">
        {crear.isError && err?.code !== 'VALIDACION_FALLIDA' && <MensajeError error={crear.error} />}
        {err?.code === 'VALIDACION_FALLIDA' && <Aviso tipo="alerta">Revisa los datos marcados en rojo.</Aviso>}
      </div>

      <div className="sticky bottom-20 z-[600] mt-4 sm:bottom-4">
        <button type="submit" className="btn-reportar w-full py-3.5 text-base shadow-[0_4px_16px_rgb(20_72_69/0.25)]" disabled={!puedeEnviar}>
          {crear.isPending ? (
            <>
              <Spinner /> Enviando y analizando la foto
            </>
          ) : (
            <>
              <Send size={18} aria-hidden /> Enviar reporte
            </>
          )}
        </button>
      </div>
    </form>
  )
}
