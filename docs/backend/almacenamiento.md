# Almacenamiento de imágenes

Las fotografías se guardan **en disco**, fuera de PostgreSQL. En la tabla `archivo`
solo quedan metadatos: ruta relativa, MIME, tamaño, variante, sha256 y fecha.

## Estructura
```
STORAGE_DIR/
├── originales/AAAA/MM/<26 caracteres aleatorios>.jpg|png|webp
└── procesadas/AAAA/MM/<26 caracteres aleatorios>.jpg|png|webp
```
- Nombre: `crypto/rand` (130 bits). El nombre que envía el usuario se ignora.
- Extensión: sale del MIME detectado por contenido.
- Escritura atómica (`.tmp` y luego rename) y permisos 0750/0640.
- Todo pasa por `os.Root`: ninguna ruta puede salir de `STORAGE_DIR`.
- La BD repite la regla con `ck_archivo_ruta`.

## Política de acceso
- Las imágenes **no son públicas**: no hay servidor estático para `STORAGE_DIR`.
- Solo el **autor del reporte** o un **administrador** pueden verlas o eliminarlas.
- A cualquier otro se le responde `404` (no `403`), para no revelar que el archivo existe.
- La descarga envía `Cache-Control: private, no-store` y `X-Content-Type-Options: nosniff`.

## Endpoints (todos con `Authorization: Bearer <token>`)
| Método | Ruta | Respuesta |
|---|---|---|
| POST | /api/v1/reportes/:id/archivos (campo `foto`) | 201 con metadatos, 400, 404, 413 |
| GET | /api/v1/archivos/:id | 200 con la imagen (admite Range), 404 |
| DELETE | /api/v1/archivos/:id | 204, 404 |

## Formatos y límites
JPEG, PNG y WebP; máximo 10 MiB (`ck_archivo_tamano`). La validación profunda
(imagen corrupta, EXIF) corresponde a la HU 04.2.

## Volumen en Docker (se declara en el Módulo 10)
```yaml
services:
  api:
    environment:
      STORAGE_DIR: /app/storage
    volumes:
      - urbia_storage:/app/storage
volumes:
  urbia_storage:
```

## Consistencia
- Si falla el INSERT, se borra el archivo recién escrito.
- Al eliminar, primero se borra el registro y después el archivo. Si el disco falla,
  se registra un `warn` con la ruta huérfana.
- Borrar un `reporte` elimina sus filas de `archivo` (`ON DELETE CASCADE`), pero
  **no** los archivos en disco: el módulo de reportes debe llamar a `ArchivoService`.