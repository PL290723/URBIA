import { Suspense } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router'
import { Camera, ClipboardList, LayoutDashboard, ListChecks, Map, UserRound, Users, type LucideIcon } from 'lucide-react'
import { useEsAdmin, useUsuario } from '../auth/useSesion'
import { useSalud } from '../api/consultas'
import { AvisoSinConexion } from '../pwa/pwa'
import { Cargando, Logo } from './ui'

interface Enlace {
  a: string
  texto: string
  Icono: LucideIcon
  fin?: boolean
}

const CIUDADANO: Enlace[] = [
  { a: '/', texto: 'Mapa', Icono: Map, fin: true },
  { a: '/mis-reportes', texto: 'Mis reportes', Icono: ClipboardList },
  { a: '/perfil', texto: 'Perfil', Icono: UserRound },
]

const ADMIN: Enlace[] = [
  { a: '/admin', texto: 'Resumen', Icono: LayoutDashboard, fin: true },
  { a: '/admin/reportes', texto: 'Reportes', Icono: ListChecks },
  { a: '/', texto: 'Mapa', Icono: Map, fin: true },
  { a: '/admin/usuarios', texto: 'Usuarios', Icono: Users },
  { a: '/perfil', texto: 'Perfil', Icono: UserRound },
]

function EstadoServicio({ compacto = false }: { compacto?: boolean }) {
  const { data, isError } = useSalud()
  const estado = isError ? 'caido' : (data?.status ?? null)
  if (!estado) return null
  const info = {
    ok: { color: 'bg-green-500', texto: 'Todo en línea' },
    degradado: { color: 'bg-amber-500', texto: 'Análisis de fotos sin servicio' },
    caido: { color: 'bg-red-500', texto: 'La API no responde' },
  }[estado]
  return (
    <span className="inline-flex items-center gap-2 text-xs text-gris" title={info.texto}>
      <span className={`size-2 shrink-0 rounded-full ${info.color}`} />
      {!compacto && info.texto}
    </span>
  )
}

/** Barra inferior en celular. El botón de reportar va al centro, en amarillo vial. */
function BarraInferior({ enlaces, conReportar }: { enlaces: Enlace[]; conReportar: boolean }) {
  const items: (Enlace | 'reportar')[] = conReportar ? [enlaces[0], enlaces[1], 'reportar', enlaces[2]] : enlaces
  return (
    <nav className="pb-segura fixed inset-x-0 bottom-0 z-[1000] border-t border-linea bg-white sm:hidden" aria-label="Navegación">
      <ul className="flex h-16 items-stretch">
        {items.map((e) =>
          e === 'reportar' ? (
            <li key="reportar" className="flex flex-1 items-center justify-center">
              <NavLink to="/reportar" className="btn-reportar size-12 rounded-full p-0" aria-label="Reportar un problema">
                <Camera size={22} strokeWidth={2} aria-hidden />
              </NavLink>
            </li>
          ) : (
            <li key={e.a + e.texto} className="flex-1">
              <NavLink
                to={e.a}
                end={e.fin}
                className={({ isActive }) =>
                  `flex h-full flex-col items-center justify-center gap-1 text-[11px] font-medium ${
                    isActive ? 'text-urbia-800' : 'text-gris-claro'
                  }`
                }
              >
                {({ isActive }) => (
                  <>
                    <e.Icono size={22} strokeWidth={isActive ? 2.25 : 1.75} aria-hidden />
                    {e.texto}
                  </>
                )}
              </NavLink>
            </li>
          ),
        )}
      </ul>
    </nav>
  )
}

function LayoutCiudadano() {
  const usuario = useUsuario()
  const { pathname } = useLocation()
  const mapa = pathname === '/'

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="pt-segura sticky top-0 z-[1000] border-b border-linea bg-white">
        <div className="mx-auto flex h-14 max-w-5xl items-center gap-6 px-4">
          <NavLink to="/" className="flex items-center gap-2" aria-label="URBIA, inicio">
            <Logo className="h-8 w-auto" />
            <span className="titulo text-xl tracking-wide text-urbia-800">URBIA</span>
          </NavLink>
          <nav className="hidden h-full items-stretch gap-1 sm:flex" aria-label="Principal">
            {CIUDADANO.map((e) => (
              <NavLink
                key={e.a}
                to={e.a}
                end={e.fin}
                className={({ isActive }) =>
                  `flex items-center gap-2 border-b-2 px-3 text-sm font-medium ${
                    isActive ? 'border-urbia-800 text-urbia-800' : 'border-transparent text-gris hover:text-asfalto'
                  }`
                }
              >
                <e.Icono size={18} aria-hidden />
                {e.texto}
              </NavLink>
            ))}
          </nav>
          <div className="ml-auto flex items-center gap-4">
            <span className="hidden text-sm text-gris md:block">{usuario?.nombre}</span>
            <NavLink to="/reportar" className="btn-reportar hidden sm:inline-flex">
              <Camera size={18} aria-hidden /> Reportar
            </NavLink>
          </div>
        </div>
        <AvisoSinConexion />
      </header>

      <main className={mapa ? 'relative flex-1' : 'mx-auto w-full max-w-5xl flex-1 px-4 pt-5 pb-24 sm:pb-10'}>
        <Suspense fallback={<Cargando />}>
          <Outlet />
        </Suspense>
      </main>
      <BarraInferior enlaces={CIUDADANO} conReportar />
    </div>
  )
}

/** Administración: barra lateral en escritorio, barra inferior en celular. */
function LayoutAdmin() {
  const usuario = useUsuario()
  const { pathname } = useLocation()
  const mapa = pathname === '/'

  return (
    <div className="min-h-dvh lg:grid lg:grid-cols-[15rem_1fr] lg:bg-[linear-gradient(to_right,#fff_15rem,transparent_15rem)]">
      <aside className="sticky top-0 hidden h-dvh flex-col border-r border-linea bg-white lg:flex">
        <NavLink to="/admin" className="flex items-center gap-2 px-5 pt-5 pb-6">
          <Logo className="h-9 w-auto" />
          <span>
            <span className="titulo block text-xl tracking-wide text-urbia-800">URBIA</span>
            <span className="block text-xs text-gris">Administración</span>
          </span>
        </NavLink>
        <nav className="flex flex-col gap-0.5 px-3" aria-label="Principal">
          {ADMIN.map((e) => (
            <NavLink
              key={e.a + e.texto}
              to={e.a}
              end={e.fin}
              className={({ isActive }) =>
                `flex items-center gap-3 rounded-[10px] px-3 py-2.5 text-sm font-medium ${
                  isActive ? 'bg-urbia-800 text-white' : 'text-gris hover:bg-concreto hover:text-asfalto'
                }`
              }
            >
              <e.Icono size={18} aria-hidden />
              {e.texto}
            </NavLink>
          ))}
        </nav>
        <div className="mt-auto space-y-2 border-t border-linea px-5 py-4">
          <EstadoServicio />
          <p className="truncate text-sm font-medium">{usuario?.nombre}</p>
        </div>
      </aside>

      <div className="flex min-h-dvh flex-col">
        <header className="pt-segura sticky top-0 z-[1000] border-b border-linea bg-white lg:hidden">
          <div className="flex h-14 items-center gap-2 px-4">
            <Logo className="h-8 w-auto" />
            <span className="titulo text-xl tracking-wide text-urbia-800">URBIA</span>
            <span className="text-sm text-gris">Administración</span>
            <span className="ml-auto">
              <EstadoServicio compacto />
            </span>
          </div>
          <AvisoSinConexion />
        </header>
        <div className="hidden lg:block">
          <AvisoSinConexion />
        </div>
        <main className={mapa ? 'relative flex-1' : 'w-full max-w-6xl flex-1 px-4 pt-5 pb-24 sm:px-6 lg:px-8 lg:pt-8 lg:pb-10'}>
          <Suspense fallback={<Cargando />}>
            <Outlet />
          </Suspense>
        </main>
        <BarraInferior enlaces={ADMIN} conReportar={false} />
      </div>
    </div>
  )
}

export function Layout() {
  return useEsAdmin() ? <LayoutAdmin /> : <LayoutCiudadano />
}
