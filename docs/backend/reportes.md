# Creación de reportes (HU 06.1)

Un ciudadano autenticado registra un problema urbano con **fotografía, ubicación y
descripción**. El reporte nace en estado `pendiente`.

## Endpoint

`POST /api/v1/reportes` · `Authorization: Bearer <token>` · `multipart/form-data`

| Campo | Tipo | Obligatorio | Regla |
|---|---|---|---|
| `foto` | archivo | sí | JPEG, PNG o WebP; máx. 10 MB; 200–8000 px por lado |
| `descripcion` | texto | sí | 10 a 1000 caracteres (sin contar espacios de los extremos) |
| `latitud` | decimal | sí | −90 a 90 (WGS 84) |
| `longitud` | decimal | sí | −180 a 180 (WGS 84) |
| `origen_ubicacion` | texto | sí | `gps` o `manual` |
| `precision_m` | decimal | solo si es `gps` | mayor que 0 y hasta 100 m |
| `id_categoria` | entero | no | categoría existente y activa |

- El autor **sale del token**; no se acepta `id_usuario` en el formulario.
- Solo el rol `ciudadano` puede crear reportes (`dominio.Rol.PuedeCrearReporte`).
- La categoría es opcional porque el modelo de visión (módulo 05) o un administrador
  pueden asignarla después.

## Ubicación

- Se guarda como `GEOMETRY(Point, 4326)` con `ST_SetSRID(ST_MakePoint(lon, lat), 4326)`.
  Ojo: PostGIS recibe **(longitud, latitud)**.
- Las coordenadas se redondean a 6 decimales (~11 cm).
- **Sin ubicación** (no llegan latitud ni longitud): 400 con `campo: "ubicacion"`, para
  que la app pida activar el GPS o marcar el punto en el mapa.
- **(0, 0)** se rechaza (`sin_senal`): es lo que mandan muchos GPS sin señal.
- **GPS impreciso** (`precision_m` > 100): 400 con `regla: "precision_insuficiente"`; la
  app debe ofrecer marcar el punto manualmente.
- **Ubicación manual** (`origen_ubicacion=manual`): el ciudadano marcó el punto en el
  mapa porque el GPS no estaba disponible. `precision_m` se ignora y queda `NULL`.
- La ubicación del EXIF de la foto **no** se usa (se elimina al sanear la imagen).

Migración `00005_ubicacion_reporte.sql`: columnas `origen_ubicacion` y `precision_m`
con los CHECK `ck_reporte_origen_ubicacion`, `ck_reporte_precision` y
`ck_reporte_precision_origen` (gps ⇒ con precisión, manual ⇒ sin precisión).

## Flujo y consistencia

1. Validar campos (todos los errores se devuelven juntos).
2. Comprobar que la categoría exista y esté activa.
3. Validar y sanear la foto (`imagenes.Saneador`) y escribirla en `originales/`.
4. **Una transacción**: `INSERT reporte` (estado `pendiente`) → `INSERT archivo` →
   `INSERT historial_estado` (`NULL → pendiente`).
5. Si la transacción falla, se hace rollback y se **borra la foto del disco**.

## Respuestas

| Código | Cuándo |
|---|---|
| 201 | Creado. Header `Location: /api/v1/reportes/{id}` y el resumen en `data` |
| 400 `VALIDACION_FALLIDA` | Campos faltantes o inválidos; `details` = `[{campo, regla, mensaje}]` |
| 400 `SOLICITUD_INVALIDA` | El cuerpo no es `multipart/form-data` o el archivo está vacío |
| 401 | Sin token, token inválido o la cuenta ya no existe |
| 403 | El rol no es `ciudadano` |
| 413 | La foto supera 10 MB |
| 415 | Formato de imagen no permitido |
| 422 `IMAGEN_INVALIDA` | Imagen dañada o con dimensiones fuera de rango |

Ejemplo de `201`:

```json
{
  "success": true,
  "data": {
    "id_reporte": 12,
    "estado": "pendiente",
    "prioridad": "media",
    "descripcion": "Bache profundo frente a la escuela",
    "categoria": { "id_categoria": 1, "nombre": "bache" },
    "ubicacion": { "latitud": 20.5235, "longitud": -100.8157, "origen": "gps", "precision_m": 12.5 },
    "foto": {
      "id_archivo": 30, "id_reporte": 12, "tipo": "imagen", "variante": "original",
      "mime": "image/jpeg", "tamano": 184233, "fecha_subida": "2026-11-16T03:10:00Z"
    },
    "fecha_creacion": "2026-11-16T03:10:00Z"
  }
}
```