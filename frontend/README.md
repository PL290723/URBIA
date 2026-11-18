# URBIA — Frontend (PWA)

Página web responsiva que en el celular se puede **instalar como app**. Consume la API Go
de `backend-go/` según su contrato `backend-go/internal/apidocs/openapi.yaml`.

| Necesidad | Herramienta |
|---|---|
| Base | Vite 8 + React 19 + TypeScript 5.9 |
| Instalable como app | `vite-plugin-pwa` (manifest, íconos, service worker, aviso de actualización) |
| Estilos responsivos | Tailwind CSS 4 (paleta `urbia-*` tomada del logo) |
| Mapa de reportes | Leaflet + react-leaflet con OpenStreetMap |
| Llamadas a la API y caché | TanStack Query 5 |
| Tipos de la API | `openapi-typescript` → `src/api/schema.d.ts` |
| Gráficas del panel admin | Recharts 3 |
| Rutas | React Router 8 |

## Inicio rápido (Windows, PowerShell)

Requisitos: Node.js 22 LTS o superior y la API corriendo (`deployments/`, puerto 8080).

```powershell
cd frontend
npm install
Copy-Item .env.example .env
npm run dev
```

Abrir <http://localhost:5173>. Cuentas de prueba: las de `deployments/scripts/cuentas-demo.ps1`.

| Script | Qué hace |
|---|---|
| `npm run dev` | Servidor de desarrollo con recarga en caliente (sin service worker) |
| `npm run build` | Revisa tipos y genera `dist/` con el service worker |
| `npm run preview` | Sirve `dist/` como en producción (PWA completa) en el puerto 5173 |
| `npm run gen:api` | Regenera los tipos desde `openapi.yaml` (correr cada vez que cambie el contrato) |
| `npm run typecheck` / `npm run lint` | TypeScript estricto / oxlint |

## Cómo llega a la API

El frontend llama a `/api/...` en su **misma dirección** y Vite lo reenvía a
`VITE_PROXY_TARGET` (por defecto `http://localhost:8080`), tanto en `dev` como en `preview`.

- El celular solo necesita llegar al puerto **5173** del equipo servidor.
- No hay CORS (es el mismo origen) ni "contenido mixto" si el frontend usa HTTPS.
- Si se publica el build en otro servidor, definir `VITE_API_URL` y agregar ese origen a
  `CORS_ALLOWED_ORIGINS` de la API.

## Celulares en la red local (feria)

1. Seguir `deployments/feria.md` (red privada, IP del equipo).
2. Firewall: permitir también el puerto del frontend (como administrador):
   ```powershell
   New-NetFirewallRule -DisplayName "URBIA Frontend (feria)" -Direction Inbound -Protocol TCP -LocalPort 5173 -Action Allow -Profile Private
   ```
3. `npm run build` y `npm run preview`; en el celular abrir `http://<IP>:5173`.

### GPS e instalación necesitan HTTPS

Los navegadores solo permiten el **GPS** y el **service worker** (instalación completa y
modo sin conexión) en `https://` o en `localhost`. Por `http://<IP>:5173` la app funciona
igual, pero la ubicación se marca tocando el mapa (la API la recibe con
`origen_ubicacion = manual`). Para tener GPS en los celulares de la demo hay dos opciones:

**Opción A — certificado local con mkcert (recomendada):**

```powershell
winget install FiloSottile.mkcert
mkcert -install
mkdir certs
mkcert -cert-file certs/urbia.pem -key-file certs/urbia-key.pem localhost 127.0.0.1 <IP>
```

Descomentar `URBIA_HTTPS_CERT` y `URBIA_HTTPS_KEY` en `.env`, volver a iniciar y abrir
`https://<IP>:5173`. En cada celular instalar el certificado raíz que muestra
`mkcert -CAROOT` (`rootCA.pem`): Android → Ajustes → Seguridad → Instalar certificado → CA;
iPhone → abrir el archivo, instalar el perfil y activarlo en Ajustes → General → Información →
Confianza de certificados. Si la IP cambia, generar el certificado de nuevo.

**Opción B — solo Chrome en Android, sin certificados:** en `chrome://flags` activar
*Insecure origins treated as secure* con `http://<IP>:5173` y reiniciar Chrome.

### Mapa sin internet

Los mosaicos de OpenStreetMap vienen de internet. El service worker guarda los que ya se
vieron (hasta 3000, 30 días): antes de la feria, con internet, recorrer la zona de la demo en
el mapa desde cada celular. Las fotos y la API nunca se guardan en caché.

## Estructura

```
frontend/
├── public/                 Logo, íconos PWA (192, 512, maskable, apple-touch) y favicon
├── src/
│   ├── api/
│   │   ├── schema.d.ts     Tipos generados (no editar)
│   │   ├── tipos.ts        Alias legibles de los tipos del contrato
│   │   ├── client.ts       fetch + sobre {success,data|error} → ApiError
│   │   └── consultas.ts    Un hook de TanStack Query por endpoint
│   ├── auth/               Sesión (JWT en localStorage) y rutas protegidas por rol
│   ├── components/         Layout, mapas Leaflet, foto protegida, piezas de UI
│   ├── lib/                Catálogos, formatos de fecha, GPS, reducción de fotos
│   ├── pages/              Login/registro, mapa, nuevo reporte, confirmar categoría,
│   │   │                   mis reportes, detalle, perfil
│   │   └── admin/          Panel con estadísticas, bandeja de reportes, usuarios y
│   │                       herramientas del detalle (estado, prioridad, duplicados)
│   └── pwa/                Registro del service worker, instalar app, sin conexión
└── vite.config.ts          Proxy a la API, HTTPS opcional y configuración PWA
```

## Pantallas por rol

| Ruta | Ciudadano | Administrador |
|---|---|---|
| `/` mapa de reportes del área visible | ✔ (ajenos con ubicación aproximada) | ✔ |
| `/reportar` foto + ubicación + descripción | ✔ | — |
| `/reportes/:id/categoria` aceptar o corregir la sugerencia de la IA | ✔ | — |
| `/mis-reportes` | ✔ | — |
| `/reportes/:id` detalle, foto e historial | ✔ | ✔ + estado, prioridad y duplicados |
| `/admin` estadísticas (periodo, estado, categoría, mapa agregado) | — | ✔ |
| `/admin/reportes` bandeja con filtros en la URL | — | ✔ |
| `/admin/usuarios` | — | ✔ |

## Notas y limitaciones

- **Categorías:** la API aún no tiene `GET /categorias`; el catálogo está en
  `src/lib/catalogos.ts` copiado de `database/migrations/00002_datos_semilla.sql`.
  Si se agrega una categoría en la base, agregarla también ahí.
- **Fotos:** `GET /api/v1/archivos/{id}` pide el token, así que se descargan con `fetch`
  y se muestran como *object URL*. Antes de subir, las fotos se reducen a 2560 px (JPEG 85 %).
- **Sesión:** el token se guarda en `localStorage` para que la app instalada no pida login
  cada vez; al vencer (`JWT_TTL`) o con un 401 se cierra la sesión y se limpia la caché.
- `npm audit` reporta avisos moderados en `glob`/`minimatch` dentro de `workbox-build`
  (dependencia de `vite-plugin-pwa`). Solo se usan al compilar, no llegan al navegador.
