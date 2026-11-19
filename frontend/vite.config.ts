import { readFileSync } from 'node:fs'
import { defineConfig, loadEnv, type ServerOptions } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

// Configuración de Vite para el frontend de URBIA.
//
// - El frontend llama a la API en la MISMA dirección (/api/...) y Vite la reenvía a la
//   API Go (VITE_PROXY_TARGET). Así el celular solo necesita llegar al puerto 5173 y no
//   hay CORS ni "contenido mixto" si se usa HTTPS.
// - HTTPS opcional (URBIA_HTTPS_CERT / URBIA_HTTPS_KEY): el GPS y el service worker
//   solo funcionan en "contextos seguros" (https o localhost). Ver frontend/README.md.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const destinoApi = env.VITE_PROXY_TARGET || 'http://localhost:8080'

  let https: ServerOptions['https']
  if (env.URBIA_HTTPS_CERT && env.URBIA_HTTPS_KEY) {
    https = {
      cert: readFileSync(env.URBIA_HTTPS_CERT),
      key: readFileSync(env.URBIA_HTTPS_KEY),
    }
  }

  const servidor: ServerOptions = {
    host: true, // escucha en la red local para que entren los celulares
    port: 5173,
    strictPort: true,
    // Dominios públicos que pueden abrir la app: túnel de Cloudflare y, si hay, dominios propios
    // en URBIA_HOSTS_PERMITIDOS (separados por coma). Ver "Acceso desde internet" en README.md.
    allowedHosts: [
      '.trycloudflare.com',
      ...(env.URBIA_HOSTS_PERMITIDOS ? env.URBIA_HOSTS_PERMITIDOS.split(',').map((h) => h.trim()) : []),
    ],
    https,
    proxy: {
      '/api': { target: destinoApi, changeOrigin: true },
    },
  }

  return {
    plugins: [
      react(),
      tailwindcss(),
      VitePWA({
        registerType: 'prompt',
        injectRegister: false, // se registra desde src/pwa/ (aviso de actualización)
        includeAssets: ['favicon.ico', 'apple-touch-icon-180x180.png', 'logo-urbia.png', 'logo-marca.png'],
        manifest: {
          id: '/',
          name: 'URBIA — Detección inteligente de problemas urbanos',
          short_name: 'URBIA',
          description:
            'Reporta baches, basura, fugas y otros problemas urbanos con una foto y tu ubicación.',
          lang: 'es-MX',
          start_url: '/',
          scope: '/',
          display: 'standalone',
          orientation: 'portrait',
          background_color: '#f2f4f1',
          theme_color: '#144845',
          categories: ['utilities', 'government'],
          icons: [
            { src: 'pwa-64x64.png', sizes: '64x64', type: 'image/png' },
            { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
            { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
            { src: 'maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
          ],
          shortcuts: [
            { name: 'Nuevo reporte', short_name: 'Reportar', url: '/reportar' },
            { name: 'Mis reportes', short_name: 'Mis reportes', url: '/mis-reportes' },
          ],
        },
        workbox: {
          globPatterns: ['**/*.{js,css,html,ico,png,svg,webmanifest,woff2}'],
          navigateFallback: '/index.html',
          // La API nunca se guarda en caché del service worker (datos privados con token)
          navigateFallbackDenylist: [/^\/api\//],
          runtimeCaching: [
            {
              // Mosaicos de OpenStreetMap: se guardan para que el mapa funcione sin internet
              // en las zonas que ya se vieron (útil en la feria).
              urlPattern: /^https:\/\/[abc]\.tile\.openstreetmap\.org\/.*/i,
              handler: 'CacheFirst',
              options: {
                cacheName: 'urbia-mosaicos-osm',
                expiration: { maxEntries: 3000, maxAgeSeconds: 60 * 60 * 24 * 30 },
                cacheableResponse: { statuses: [0, 200] },
              },
            },
          ],
        },
        devOptions: { enabled: false },
      }),
    ],
    define: {
      __VERSION__: JSON.stringify(process.env.npm_package_version ?? '0.1.0'),
    },
    server: servidor,
    preview: servidor,
  }
})
