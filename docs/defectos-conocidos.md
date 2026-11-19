# Defectos conocidos (v0.1.0-rc.1)

Ninguno bloquea la demostración. Cada uno tiene una alternativa para la feria. El detalle de
seguridad está en [backend/seguridad.md](backend/seguridad.md) y los problemas de operación en
[solucion-problemas.md](solucion-problemas.md).

| ID | Defecto | Impacto | Alternativa en la demo |
|---|---|---|---|
| D01 | Con Docker Desktop todas las peticiones llegan a la API con la misma IP | El límite de login es compartido: 429 para todos | `LOGIN_MAX_INTENTOS=50`, `LOGIN_VENTANA=2m` |
| D02 | Swagger UI (`/api/v1/docs`) se carga desde un CDN | Sin internet la página queda en blanco | `GET /api/v1/openapi.yaml` o Bruno |
| D03 | Los mosaicos del mapa vienen de OpenStreetMap | Sin internet, zonas no visitadas se ven en gris | Recorrer la zona de la demo antes, con internet |
| D04 | GPS e instalación de la PWA requieren HTTPS | Por `http://<IP>:5173` no hay GPS | Marcar la ubicación en el mapa (`manual`), mkcert o túnel |
| D05 | No existe `GET /categorias` | El catálogo está copiado en `frontend/src/lib/catalogos.ts` | Mantener ambos iguales si se agrega una categoría |
| D06 | La IA solo detecta `bache` y `basura`; recall de bache 0.65 | A veces no sugiere o sugiere mal | El ciudadano confirma o corrige la categoría |
| D07 | Borrar un reporte no borra sus fotos del disco | Archivos huérfanos en el volumen | Limpieza manual; no afecta la demo |
| D08 | Sin revocación de tokens ni cambio de contraseña | Un token vale hasta `JWT_TTL` (2 h) | `JWT_SECRET` nuevo para la feria |
| D09 | Los administradores se promueven a mano | Paso manual | `deployments/scripts/cuentas-demo.ps1` |
| D10 | HTTP sin TLS en la red local | El token viaja en claro | Red propia con WPA2/WPA3 |
| D11 | Las fotos pueden mostrar rostros o placas | Privacidad | Fotos privadas (solo autor y administrador); usar fotos de prueba |
| D12 | Pesos de la fórmula de prioridad provisionales | La prioridad es orientativa | Se muestra como `v1-provisional` |
| D13 | El túnel de Cloudflare publica app y API en internet | Cualquiera con la URL puede registrarse | Solo durante la demo; cerrarlo al terminar |
| D14 | Una instancia por servicio y respaldos manuales | Sin alta disponibilidad | `respaldar.ps1` antes y después de la feria |
| D15 | `npm audit`: avisos moderados en `glob`/`minimatch` (de `workbox-build`) | Solo en compilación, no llegan al navegador | Ninguna necesaria |
