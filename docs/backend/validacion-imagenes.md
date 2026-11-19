# Validación de imágenes recibidas

Cada foto que llega a `POST /api/v1/reportes/:id/archivos` pasa por `imagenes.Saneador`
antes de guardarse. Las validaciones van en este orden:

| # | Validación | Si falla |
|---|---|---|
| 1 | Nombre de archivo sin `..`, separadores, `:` ni caracteres de control | 400 `SOLICITUD_INVALIDA` |
| 2 | Tamaño ≤ 10 MiB y no vacío | 413 `ARCHIVO_DEMASIADO_GRANDE` / 400 |
| 3 | Firma real del contenido (magic bytes): JPEG, PNG o WebP | 415 `FORMATO_NO_PERMITIDO` |
| 4 | La cabecera decodifica y el formato coincide con la firma | 422 `IMAGEN_INVALIDA` |
| 5 | Dimensiones: 200–8000 px por lado, máx. 25 MP (antes de decodificar) | 422 `IMAGEN_INVALIDA` |
| 6 | Decodificación completa (archivos truncados o corruptos) | 422 `IMAGEN_INVALIDA` |

Nunca se confía en la extensión, el nombre ni el `Content-Type` que envía el cliente.

## Formatos acordados
JPEG, PNG y WebP. Se rechazan GIF, HEIC/HEIF, BMP, TIFF, SVG y cualquier otro.
En iPhone, Safari convierte HEIC a JPEG al subir desde un `<input type="file">`.

## Metadatos EXIF y datos sensibles
Las fotos de celular traen EXIF con **coordenadas GPS** (pueden revelar el domicilio
del ciudadano), modelo del teléfono y fecha. Política:
- Toda foto original se **re-codifica a JPEG (calidad 90)**, lo que elimina EXIF, XMP,
  IPTC, comentarios y perfiles ICC.
- Antes de eliminar el EXIF se aplica su `Orientation`, para que la foto no quede girada.
- La transparencia de los PNG se aplana sobre fondo blanco.
- La ubicación oficial del reporte es la que manda el ciudadano (`reporte.ubicacion`),
  no la del EXIF.
- Por eso `archivo.mime` es siempre `image/jpeg` y `sha256` corresponde al archivo saneado.

## Recursos
Máximo 2 imágenes se decodifican a la vez (una foto de 25 MP usa ~100 MB de RAM).
Los rechazos se registran en el log (nivel info) con el motivo, sin guardar el contenido.