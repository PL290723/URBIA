// Inicio de sesión y registro de ciudadanos.
import { useState, type FormEvent, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { useLogin, useRegistro } from '../api/consultas'
import { ApiError } from '../api/client'
import { tomarMotivoCierre } from '../auth/almacen'
import { Aviso, Logo, MensajeError, Spinner } from '../components/ui'
import { BotonInstalar } from '../pwa/pwa'

function Marco({ children }: { children: ReactNode }) {
  return (
    <div className="pt-segura grid min-h-dvh lg:grid-cols-[1.1fr_1fr]">
      {/* Columna de presentación (solo escritorio) */}
      <section className="relative hidden overflow-hidden bg-urbia-800 text-white lg:flex lg:flex-col lg:justify-between lg:p-12">
        <div className="flex items-center gap-3">
          <span className="rounded-xl bg-white p-2">
            <Logo className="h-10 w-auto" />
          </span>
          <span className="titulo text-2xl tracking-wide">URBIA</span>
        </div>
        <div className="max-w-md">
          <h2 className="titulo text-[3.25rem] text-white">Lo que ves en tu calle, en el mapa de la ciudad.</h2>
          <p className="mt-5 text-lg text-urbia-100">
            Toma una foto del bache, la fuga o la luminaria apagada. URBIA reconoce el problema y la administración
            le da seguimiento hasta resolverlo.
          </p>
        </div>
        <p className="text-sm text-urbia-200">Feria de Proyectos TecNM Celaya 2026</p>
        {/* Línea de carril: el único adorno, tomado de la pintura vial */}
        <div className="pointer-events-none absolute top-0 right-16 h-full w-2 bg-[repeating-linear-gradient(to_bottom,#f2c230_0_48px,transparent_48px_88px)] opacity-90" aria-hidden />
      </section>

      <section className="flex flex-col items-center justify-center px-5 py-10">
        <Logo variante="completo" className="mb-8 w-52 lg:hidden" />
        <div className="w-full max-w-sm">{children}</div>
        <BotonInstalar className="mt-8 w-full max-w-sm" />
      </section>
    </div>
  )
}

function Campo({
  etiqueta,
  error,
  ...props
}: { etiqueta: string; error?: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="block">
      <span className="etiqueta">{etiqueta}</span>
      <input className="campo" aria-invalid={!!error} {...props} />
      {error && <span className="mt-1.5 block text-sm text-red-700">{error}</span>}
    </label>
  )
}

export function Login() {
  const login = useLogin()
  const navegar = useNavigate()
  const { state } = useLocation() as { state?: { desde?: string } }
  const [motivo] = useState(tomarMotivoCierre)
  const [correo, setCorreo] = useState('')
  const [contrasena, setContrasena] = useState('')

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    login.mutate(
      { correo: correo.trim(), contrasena },
      {
        onSuccess: (r) =>
          navegar(state?.desde ?? (r.usuario.rol === 'administrador' ? '/admin' : '/'), { replace: true }),
      },
    )
  }

  return (
    <Marco>
      <h1 className="titulo text-3xl">Inicia sesión</h1>
      <p className="mt-1 mb-6 text-gris">Entra para reportar y ver el avance de tus reportes.</p>
      {motivo === 'expirada' && (
        <div className="mb-5">
          <Aviso tipo="alerta">Tu sesión terminó por tiempo. Vuelve a iniciar sesión.</Aviso>
        </div>
      )}
      <form onSubmit={enviar} className="space-y-5" noValidate>
        <Campo
          etiqueta="Correo"
          type="email"
          autoComplete="email"
          inputMode="email"
          required
          value={correo}
          onChange={(e) => setCorreo(e.target.value)}
        />
        <Campo
          etiqueta="Contraseña"
          type="password"
          autoComplete="current-password"
          required
          value={contrasena}
          onChange={(e) => setContrasena(e.target.value)}
        />
        {login.isError && <MensajeError error={login.error} />}
        <button type="submit" className="btn-primario w-full" disabled={login.isPending || !correo || !contrasena}>
          {login.isPending && <Spinner />} Iniciar sesión
        </button>
      </form>
      <p className="mt-6 text-center text-gris">
        ¿Primera vez?{' '}
        <Link to="/registro" className="btn-texto">
          Crea una cuenta
        </Link>
      </p>
    </Marco>
  )
}

export function Registro() {
  const registro = useRegistro()
  const navegar = useNavigate()
  const [nombre, setNombre] = useState('')
  const [correo, setCorreo] = useState('')
  const [contrasena, setContrasena] = useState('')
  const [confirmacion, setConfirmacion] = useState('')
  const [local, setLocal] = useState<Record<string, string>>({})

  const err = registro.error instanceof ApiError ? registro.error : null
  const errorDe = (c: string) => local[c] ?? err?.errorDe(c)

  const enviar = (e: FormEvent) => {
    e.preventDefault()
    const errores: Record<string, string> = {}
    if (!nombre.trim()) errores.nombre = 'Escribe tu nombre'
    if (!/^\S+@\S+\.\S+$/.test(correo.trim())) errores.correo = 'Escribe un correo válido'
    if (contrasena.length < 8) errores.contrasena = 'Mínimo 8 caracteres'
    if (contrasena.length > 72) errores.contrasena = 'Máximo 72 caracteres'
    if (confirmacion !== contrasena) errores.confirmacion = 'Las contraseñas no coinciden'
    setLocal(errores)
    if (Object.keys(errores).length) return
    registro.mutate(
      { nombre: nombre.trim(), correo: correo.trim(), contrasena },
      { onSuccess: () => navegar('/', { replace: true }) },
    )
  }

  return (
    <Marco>
      <h1 className="titulo text-3xl">Crea tu cuenta</h1>
      <p className="mt-1 mb-6 text-gris">Con tu nombre y un correo puedes empezar a reportar.</p>
      <form onSubmit={enviar} className="space-y-5" noValidate>
        <Campo
          etiqueta="Nombre"
          autoComplete="name"
          maxLength={100}
          value={nombre}
          error={errorDe('nombre')}
          onChange={(e) => setNombre(e.target.value)}
        />
        <Campo
          etiqueta="Correo"
          type="email"
          inputMode="email"
          autoComplete="email"
          maxLength={150}
          value={correo}
          error={errorDe('correo')}
          onChange={(e) => setCorreo(e.target.value)}
        />
        <Campo
          etiqueta="Contraseña"
          placeholder="Mínimo 8 caracteres"
          type="password"
          autoComplete="new-password"
          maxLength={72}
          value={contrasena}
          error={errorDe('contrasena')}
          onChange={(e) => setContrasena(e.target.value)}
        />
        <Campo
          etiqueta="Confirmar contraseña"
          type="password"
          autoComplete="new-password"
          value={confirmacion}
          error={local.confirmacion}
          onChange={(e) => setConfirmacion(e.target.value)}
        />
        {registro.isError && err?.code !== 'VALIDACION_FALLIDA' && <MensajeError error={registro.error} />}
        <button type="submit" className="btn-primario w-full" disabled={registro.isPending}>
          {registro.isPending && <Spinner />} Crear cuenta
        </button>
      </form>
      <p className="mt-6 text-center text-gris">
        ¿Ya tienes cuenta?{' '}
        <Link to="/login" className="btn-texto">
          Inicia sesión
        </Link>
      </p>
    </Marco>
  )
}
