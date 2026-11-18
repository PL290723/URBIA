import { Check } from 'lucide-react'
import type { Estado } from '../api/tipos'
import { ESTADOS, ESTADO_INFO } from '../lib/catalogos'

/** Los cuatro estados del flujo en línea, como en el avance de un trámite. */
export function AvanceEstado({ estado }: { estado: Estado }) {
  const actual = ESTADOS.indexOf(estado)
  return (
    <ol className="flex items-start" aria-label={`Estado: ${ESTADO_INFO[estado].etiqueta}`}>
      {ESTADOS.map((e, i) => {
        const hecho = i < actual
        const esActual = i === actual
        return (
          <li key={e} className="relative flex flex-1 flex-col items-center text-center" aria-current={esActual ? 'step' : undefined}>
            {i > 0 && (
              <span
                className={`absolute top-3 right-1/2 h-0.5 w-full -translate-y-1/2 ${i <= actual ? 'bg-urbia-800' : 'bg-linea'}`}
                aria-hidden
              />
            )}
            <span
              className={`relative z-10 flex size-6 items-center justify-center rounded-full ${
                hecho ? 'bg-urbia-800 text-white' : esActual ? 'bg-white ring-[3px] ring-urbia-800' : 'bg-white ring-2 ring-linea'
              }`}
            >
              {hecho && <Check size={14} strokeWidth={3} aria-hidden />}
              {esActual && <span className="size-2.5 rounded-full" style={{ background: ESTADO_INFO[e].color }} />}
            </span>
            <span className={`mt-2 text-xs leading-tight ${esActual ? 'font-semibold text-asfalto' : 'text-gris'}`}>
              {ESTADO_INFO[e].etiqueta}
            </span>
          </li>
        )
      })}
    </ol>
  )
}
