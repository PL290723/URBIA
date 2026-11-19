/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/react" />

interface ImportMetaEnv {
  /** URL completa de la API si no se usa el proxy de Vite (vacío = misma dirección). */
  readonly VITE_API_URL?: string
  readonly VITE_MAPA_LAT?: string
  readonly VITE_MAPA_LON?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

/** Versión de package.json (la inyecta vite.config.ts). */
declare const __VERSION__: string
