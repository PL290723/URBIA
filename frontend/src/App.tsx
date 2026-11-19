import { lazy, Suspense } from 'react'
import { BrowserRouter, Link, Route, Routes } from 'react-router'
import { MapPinOff } from 'lucide-react'
import { RutaProtegida, RutaPublica } from './auth/Rutas'
import { Layout } from './components/Layout'
import { Cargando, Vacio } from './components/ui'
import { AvisoActualizacion } from './pwa/pwa'
import { Login, Registro } from './pages/Acceso'
import { Inicio } from './pages/Inicio'
import { NuevoReporte } from './pages/NuevoReporte'
import { ConfirmarCategoria } from './pages/ConfirmarCategoria'
import { MisReportes } from './pages/MisReportes'
import { DetalleReporte } from './pages/DetalleReporte'
import { Perfil } from './pages/Perfil'

// El panel admin (con Recharts) se descarga solo cuando entra un administrador
const Panel = lazy(() => import('./pages/admin/Panel').then((m) => ({ default: m.Panel })))
const AdminReportes = lazy(() => import('./pages/admin/AdminReportes').then((m) => ({ default: m.AdminReportes })))
const Usuarios = lazy(() => import('./pages/admin/Usuarios').then((m) => ({ default: m.Usuarios })))

function NoEncontrada() {
  return (
    <Vacio Icono={MapPinOff} titulo="Esta página no existe">
      <Link to="/" className="btn-texto">
        Ir al mapa
      </Link>
    </Vacio>
  )
}

export function App() {
  return (
    <BrowserRouter>
      <Suspense fallback={<Cargando />}>
        <Routes>
          <Route element={<RutaPublica />}>
            <Route path="/login" element={<Login />} />
            <Route path="/registro" element={<Registro />} />
          </Route>

          <Route element={<RutaProtegida />}>
            <Route element={<Layout />}>
              <Route index element={<Inicio />} />
              <Route path="reportes/:id" element={<DetalleReporte />} />
              <Route path="perfil" element={<Perfil />} />

              <Route element={<RutaProtegida rol="ciudadano" />}>
                <Route path="reportar" element={<NuevoReporte />} />
                <Route path="reportes/:id/categoria" element={<ConfirmarCategoria />} />
                <Route path="mis-reportes" element={<MisReportes />} />
              </Route>

              <Route path="admin" element={<RutaProtegida rol="administrador" />}>
                <Route index element={<Panel />} />
                <Route path="reportes" element={<AdminReportes />} />
                <Route path="usuarios" element={<Usuarios />} />
              </Route>

              <Route path="*" element={<NoEncontrada />} />
            </Route>
          </Route>
        </Routes>
      </Suspense>
      <AvisoActualizacion />
    </BrowserRouter>
  )
}
