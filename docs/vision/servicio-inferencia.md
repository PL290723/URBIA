# Servicio de inferencia: carga única y preprocesamiento

> **Historia de usuario 05.3 — Implementar carga única y preprocesamiento del modelo**
> Módulo 05 — Servicio de inferencia de visión artificial | Backend — MVP
> Depende de: 05.2 (modelo, ver [`modelo.md`](./modelo.md)) y 04.1 (almacenamiento, ver [`../backend/almacenamiento.md`](../backend/almacenamiento.md))

## 1. Qué hace y qué no

El servicio Python recibe la **ruta relativa** de una foto que Go ya guardó y saneó en `STORAGE_DIR` (HU 04.1), la prepara, ejecuta el modelo y devuelve un resultado estructurado.

- **Sí:** cargar el modelo una vez, leer la foto de forma segura, preprocesarla, inferir y aplicar la regla de decisión (`decision.py`, HU 05.2).
- **No:** guardar en la base de datos, cambiar el estado del reporte ni escribir o borrar fotos. Eso lo decide el backend en Go.
- **La red:** el contrato gRPC está en [`contrato-grpc.md`](./contrato-grpc.md) (HU 05.4) y el servidor con `GET /health` en [`servidor-inferencia.md`](./servidor-inferencia.md) (HU 05.5).

```
Go (04.1)  ──ruta relativa──▶  ServicioInferencia.inferir_archivo()
                                 │ 1. LectorImagenes.leer()       almacen.py
                                 │ 2. preparar()                  preprocesamiento.py
                                 │ 3. DetectorUrbia.detectar()    modelo.py   (cargado 1 vez)
                                 │ 4. interpretar()               decision.py (umbral 0.25)
                                 ▼
                            ResultadoInferencia  (mismos nombres que la tabla `deteccion`)
```

## 2. Carga única del modelo

`obtener_servicio()` (`urbia_vision/servicio.py`) crea el servicio **la primera vez** y después devuelve siempre el mismo objeto. Usa un candado con doble verificación, así que aunque varios hilos lo pidan a la vez, el modelo se carga una sola vez (lo prueba `test_carga_unica_con_varios_hilos`).

Al cargar, `DetectorUrbia.cargar()` (`urbia_vision/modelo.py`):

| # | Paso | Si falla |
|---|---|---|
| 1 | Lee y valida `config/modelo.json` (campos, versión X.Y.Z, clases, umbral) | `MODELO_INVALIDO` / `MODELO_NO_DISPONIBLE` |
| 2 | Verifica que exista el archivo de pesos | `MODELO_NO_DISPONIBLE` (indica cómo descargarlo) |
| 3 | Compara el **SHA-256** de los pesos con `modelo.json` | `MODELO_INVALIDO` |
| 4 | Carga YOLO y verifica que sus clases sean `{0: bache, 1: basura}` | `MODELO_INVALIDO` |
| 5 | Elige el dispositivo (`URBIA_DEVICE=auto` → GPU si hay, si no CPU) | — |
| 6 | **Calentamiento:** una inferencia con una imagen vacía de 640×640 | `MODELO_INVALIDO` |
| 7 | Registra en el log nombre, versión, umbral, imgsz, dispositivo, SHA-256 y tiempo | — |

Si cualquier paso falla, **el servicio no arranca**: es mejor no levantar que sugerir categorías con un modelo equivocado. Si la carga falla, la siguiente llamada a `obtener_servicio()` lo vuelve a intentar.

Para usar el modelo desde varios hilos (el pool del servidor gRPC de la 05.5), `detectar()` usa un candado: Ultralytics no es *thread-safe*.

### 2.1 Pesos del modelo

Los `.pt` no están en git. Se descargan del Release y se verifica su SHA-256:

```powershell
python -m scripts.descargar_modelo
```

Descarga `https://github.com/PL290723/URBIA/releases/download/modelo-v<version>/<archivo>` (o la URL de `URBIA_MODELO_URL`) a un archivo temporal, verifica el SHA-256 y solo entonces lo mueve a `models/`. Si el archivo ya existe con el SHA-256 correcto, no descarga nada.

### 2.2 Ultralytics "blindado"

Por defecto, Ultralytics **instala paquetes con pip** cuando le falta algo y **consulta internet** al importarse. Además reemplaza `PIL.Image.open` por una versión que, ante un archivo que Pillow no reconoce, intenta instalar `pi-heif`. Al probar el servicio con un JPEG truncado ocurrió justo eso.

Un servicio en producción no debe instalar nada ni depender de internet, así que:

- `blindar_ultralytics()` (`configuracion.py`) pone `YOLO_AUTOINSTALL=False` y `YOLO_OFFLINE=True` antes de importar Ultralytics. Se respetan si ya vienen definidas en el entorno.
- El preprocesamiento abre las imágenes con el `Image.open` **original** de Pillow y solo con el formato de la firma.
- La prueba `test_no_usa_el_image_open_de_ultralytics` falla si alguien vuelve a usar el `Image.open` modificado.

## 3. Lectura segura de la imagen (`almacen.py`)

Python **solo lee** de `STORAGE_DIR`. Antes de abrir nada:

1. La ruta debe cumplir **exactamente** el patrón que genera Go, el mismo de `ck_archivo_ruta` en la BD:
   `^(originales|procesadas)/[0-9]{4}/[0-9]{2}/[a-z2-7]{26}\.(jpg|png|webp)$`.
   Eso descarta `..`, rutas absolutas, `\`, `C:`, mayúsculas y nombres inventados.
2. Ya resuelta (siguiendo symlinks o junctions), la ruta debe quedar **dentro** de `STORAGE_DIR`.
3. Debe ser un archivo regular, no vacío y de 10 MiB como máximo. Nunca se leen más bytes que el límite.

## 4. Preprocesamiento estándar (`preprocesamiento.py`)

Toda imagen pasa por los mismos pasos. Los límites son los mismos que Go (`validacion-imagenes.md`), así que una foto aceptada por Go también la acepta Python:

| # | Paso | Detalle |
|---|---|---|
| 1 | Firma real (*magic bytes*) | Solo JPEG, PNG o WebP; la extensión no importa |
| 2 | Dimensiones de la cabecera, **antes** de decodificar | 200–8000 px por lado, máx. 25 MP |
| 3 | Decodificación completa | Detecta archivos truncados o corruptos |
| 4 | Orientación EXIF | Go ya la aplica; aquí se repite por si la imagen no pasó por Go |
| 5 | Conversión a RGB | La transparencia se aplana sobre blanco, igual que Go |
| 6 | Reducción a **1280 px** de lado mayor (`URBIA_LADO_MAX_INFERENCIA`) | Conserva la proporción; Ultralytics después la lleva a 640 (`imgsz`) con *letterbox*. Ahorra memoria con fotos de celular de 12+ MP |
| 7 | Arreglo NumPy **BGR** `uint8` | El formato que Ultralytics espera de un `ndarray` |

Se guarda la escala del paso 6 para que las **cajas se devuelvan en píxeles de la foto original** (después de aplicar la orientación).

## 5. Resultado y errores

`ResultadoInferencia` usa los mismos nombres que la tabla `deteccion`:

| Campo | Ejemplo | Nota |
|---|---|---|
| `resultado` | `detectado` / `no_detectado` / `error` | Regla de `decision.py` con el umbral de `modelo.json` |
| `clase_detectada`, `confianza` | `basura`, `0.9` | `NULL` si no es `detectado` |
| `modelo`, `version_modelo` | `urbia-yolo26s`, `1.0.0` | Para `deteccion.modelo` y `deteccion.version_modelo` |
| `umbral_confianza` | `0.25` | El umbral con el que se decidió |
| `cajas` | `[{clase, confianza, x1, y1, x2, y2}]` | Píxeles de la foto original, ordenadas por confianza |
| `ancho`, `alto` | `2560`, `1920` | Tamaño de la foto original |
| `milisegundos` | `174.9` | Tiempo total de la petición |
| `codigo_error`, `mensaje_error` | `IMAGEN_INVALIDA`, `…` | Solo si `resultado = error` |

**Errores por petición: el servicio sigue vivo.** Se devuelven como `resultado = error` con su código, se registran en el log (nivel `WARNING`) y la siguiente petición se atiende normal. Cualquier excepción inesperada también se convierte en `FALLO_INFERENCIA`, incluido *CUDA out of memory*.

| Código | Causa |
|---|---|
| `RUTA_INVALIDA` | La ruta no cumple el patrón o sale de `STORAGE_DIR` |
| `IMAGEN_NO_ENCONTRADA` | No existe, no es un archivo o está vacía |
| `IMAGEN_INVALIDA` | Formato no permitido, dañada, truncada, dimensiones fuera de rango o más de 10 MiB |
| `FALLO_INFERENCIA` | El modelo falló con esa imagen (por ejemplo, sin memoria en la GPU) |

**Errores del modelo: el servicio no arranca.** `MODELO_NO_DISPONIBLE` y `MODELO_INVALIDO` (sección 2). La HU 05.4 traducirá estos códigos a códigos gRPC.

## 6. Configuración

| Variable | Por defecto | Uso |
|---|---|---|
| `URBIA_MODELO_CONFIG` | `config/modelo.json` | Metadatos del modelo (05.2) |
| `STORAGE_DIR` | `../backend-go/storage` | Mismo directorio que usa Go. En Docker: `/app/storage` en **solo lectura** |
| `URBIA_DEVICE` | `auto` | `auto`, `cpu` o `0` (primera GPU) |
| `URBIA_MAX_BYTES` | `10485760` | 10 MiB, igual que Go |
| `URBIA_LADO_MAX_INFERENCIA` | `1280` | Lado mayor antes de inferir |
| `YOLO_AUTOINSTALL` / `YOLO_OFFLINE` | `False` / `True` | Puestas por `blindar_ultralytics()` |

Las rutas relativas se resuelven desde `inference-python/`.

## 7. Cómo probarlo

```powershell
cd inference-python
python -m scripts.descargar_modelo
python -m pytest -v
python -m scripts.probar_servicio --imagenes datasets/urbia/images/test --max 10 --invalidas
```

`probar_servicio` carga el modelo una vez, procesa todas las imágenes y comprueba en cada una que `obtener_servicio()` devuelve el mismo objeto. Al final imprime cuántas peticiones y errores hubo y el tiempo por imagen.

## 8. Evidencia

```
2026-11-15 16:39:44,794 INFO urbia.inferencia modelo cargado nombre=urbia-yolo26s version=1.0.0 umbral=0.25 imgsz=640 dispositivo=0 sha256=9102fa7191fa segundos=3.92

Modelo cargado UNA vez: urbia-yolo26s v1.0.0 | umbral 0.25 | imgsz 640 | dispositivo 0 | 3.9 s

  #  entrada                                    resultado     clase     conf cajas      ms
2026-11-15 16:39:44,841 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__000144244W_jpg.rf.d1dc6442492ffbc7bcce5f0412bfea66.jpg resultado=detectado clase=basura confianza=0.6798 cajas=2 ms=43.4
  1  garbage__000144244W_jpg.rf.d1dc6442492ffbc detectado     basura    0.68     2    43.4
2026-11-15 16:39:44,861 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__000839979_896x598_c_jpg.rf.04eba3aa5dbc551810d5b28ac9c38c53.jpg resultado=detectado clase=basura confianza=0.6948 cajas=1 ms=19.5
  2  garbage__000839979_896x598_c_jpg.rf.04eba3 detectado     basura    0.69     1    19.5
2026-11-15 16:39:44,881 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__02f330f2-9439-11e5-982d-40f2e96c8ad8_jpg.rf.af9f304e5a78fc6f84d268edef376fe3.jpg resultado=detectado clase=basura confianza=0.3202 cajas=1 ms=19.9
  3  garbage__02f330f2-9439-11e5-982d-40f2e96c8 detectado     basura    0.32     1    19.9
2026-11-15 16:39:44,905 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__07ff75e6-6799-11e5-8dd0-40f2e96c8ad8_jpg.rf.6e2fc221cd02c7e37fec6ca9af25d9f5.jpg resultado=detectado clase=basura confianza=0.5929 cajas=1 ms=22.4
  4  garbage__07ff75e6-6799-11e5-8dd0-40f2e96c8 detectado     basura    0.59     1    22.4
2026-11-15 16:39:44,927 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__0_The-rubbish-left-behind-in-Sefton-ParkPic-Andrew-Teebay_jpg.rf.e04ea6eb8b005b7c768ef3abda9389b8.jpg resultado=detectado clase=basura confianza=0.7176 cajas=1 ms=21.7
  5  garbage__0_The-rubbish-left-behind-in-Seft detectado     basura    0.72     1    21.7
2026-11-15 16:39:44,945 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__0ed6fe62-9439-11e5-982d-40f2e96c8ad8_jpg.rf.632814891b404ce212650bdc302e8852.jpg resultado=no_detectado clase=None confianza=None cajas=0 ms=17.4
  6  garbage__0ed6fe62-9439-11e5-982d-40f2e96c8 no_detectado  -            -     0    17.4
2026-11-15 16:39:44,965 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__1076103483_015339992667_1920x0_80_0_0_f714d219f1a26124d2452fce46923cc2_jpg.rf.bf57dc5ef46e5c203dbad32368654149.jpg resultado=detectado clase=basura confianza=0.7643 cajas=1 ms=19.3
  7  garbage__1076103483_015339992667_1920x0_80 detectado     basura    0.76     1    19.3
2026-11-15 16:39:44,985 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__1200x680_whatsapp_image_2021-09-26_at_10-12-35_jpg.rf.2ee83e27a1153d1e24d46b88f37923f8.jpg resultado=detectado clase=basura confianza=0.7945 cajas=1 ms=18.8
  8  garbage__1200x680_whatsapp_image_2021-09-2 detectado     basura    0.79     1    18.8
2026-11-15 16:39:45,004 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__12311590-679b-11e5-9696-40f2e96c8ad8_jpg.rf.555d843b794918eb6086292dd083538e.jpg resultado=detectado clase=basura confianza=0.868 cajas=1 ms=18.7
  9  garbage__12311590-679b-11e5-9696-40f2e96c8 detectado     basura    0.87     1    18.7
2026-11-15 16:39:45,024 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__1280x680_075_acar-notitle200325_npwwq_jpg.rf.556cb5f262517b5b56e3242a5816651a.jpg resultado=no_detectado clase=None confianza=None cajas=0 ms=19.3
 10  garbage__1280x680_075_acar-notitle200325_n no_detectado  -            -     0    19.3
2026-11-15 16:39:45,042 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__162645714460f1c438246b2_1626457144_3x2_md_jpg.rf.018f84b9098c66ebb17dd57c32cbf76f.jpg resultado=detectado clase=basura confianza=0.737 cajas=1 ms=18.4
 11  garbage__162645714460f1c438246b2_162645714 detectado     basura    0.74     1    18.4
2026-11-15 16:39:45,062 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__19aba664-679b-11e5-af8c-40f2e96c8ad8_jpg.rf.e849d6f7fb06fe9c940a99ce24305c46.jpg resultado=detectado clase=basura confianza=0.8945 cajas=1 ms=18.6
 12  garbage__19aba664-679b-11e5-af8c-40f2e96c8 detectado     basura    0.89     1    18.6
2026-11-15 16:39:45,081 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__1_jpg.rf.98fc8cbb6cf5ee88791a083680b55d75.jpg resultado=detectado clase=basura confianza=0.5305 cajas=2 ms=18.5
 13  garbage__1_jpg.rf.98fc8cbb6cf5ee88791a0836 detectado     basura    0.53     2    18.5
2026-11-15 16:39:45,101 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__1b7ab13a-943a-11e5-ba6d-40f2e96c8ad8_jpg.rf.9618d46b947be76a0f9de5bffe391621.jpg resultado=no_detectado clase=None confianza=None cajas=0 ms=18.9
 14  garbage__1b7ab13a-943a-11e5-ba6d-40f2e96c8 no_detectado  -            -     0    18.9
2026-11-15 16:39:45,120 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__1c15c44a-943a-11e5-ba6d-40f2e96c8ad8_jpg.rf.9481aa1e29241cd640157ebed339090d.jpg resultado=no_detectado clase=None confianza=None cajas=0 ms=18.8
 15  garbage__1c15c44a-943a-11e5-ba6d-40f2e96c8 no_detectado  -            -     0    18.8
2026-11-15 16:39:45,140 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__1c68ac06-6798-11e5-8c9e-40f2e96c8ad8_jpg.rf.ee444cd59bfe8d301877ff4e4c74ea7c.jpg resultado=detectado clase=basura confianza=0.6938 cajas=1 ms=18.6
 16  garbage__1c68ac06-6798-11e5-8c9e-40f2e96c8 detectado     basura    0.69     1    18.6
2026-11-15 16:39:45,160 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__2-2-1024x576_jpg.rf.032040d8463e60c94e9e76efca766ee8.jpg resultado=no_detectado clase=None confianza=None cajas=0 ms=19.9
 17  garbage__2-2-1024x576_jpg.rf.032040d8463e6 no_detectado  -            -     0    19.9
2026-11-15 16:39:45,179 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__20200924_161845-scaled_jpg.rf.46465de0a1977e4c6c6ca4fc99dffc29.jpg resultado=detectado clase=basura confianza=0.4721 cajas=1 ms=17.5
 18  garbage__20200924_161845-scaled_jpg.rf.464 detectado     basura    0.47     1    17.5
2026-11-15 16:39:45,199 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__21172353_hK9YZ_jpeg.rf.a67dfb37d620f6f00d94d69430d92d07.jpg resultado=detectado clase=basura confianza=0.5645 cajas=1 ms=19.2
 19  garbage__21172353_hK9YZ_jpeg.rf.a67dfb37d6 detectado     basura    0.56     1    19.2
2026-11-15 16:39:45,219 INFO urbia.inferencia inferencia entrada=datasets\urbia\images\test\garbage__2251501384_jpg.rf.da6182839bfca14ca480c51f6b4afb1e.jpg resultado=detectado clase=basura confianza=0.7021 cajas=1 ms=19.7
 20  garbage__2251501384_jpg.rf.da6182839bfca14 detectado     basura    0.70     1    19.7
2026-11-15 16:39:45,219 WARNING urbia.inferencia inferencia entrada=texto.jpg (no es imagen) resultado=error clase=None confianza=None cajas=0 ms=0.0 codigo=IMAGEN_INVALIDA
 21  texto.jpg (no es imagen)                   error         -            -     0     0.0  [IMAGEN_INVALIDA] Formato no permitido: solo JPEG, PNG o WebP
2026-11-15 16:39:45,219 WARNING urbia.inferencia inferencia entrada=jpeg truncado resultado=error clase=None confianza=None cajas=0 ms=0.1 codigo=IMAGEN_INVALIDA
 22  jpeg truncado                              error         -            -     0     0.1  [IMAGEN_INVALIDA] La cabecera de la imagen está dañada
2026-11-15 16:39:45,219 WARNING urbia.inferencia inferencia entrada=../../etc/passwd resultado=error clase=None confianza=None cajas=0 ms=0.0 codigo=RUTA_INVALIDA
 23  ../../etc/passwd                           error         -            -     0     0.0  [RUTA_INVALIDA] La ruta no tiene el formato de una foto guardada por URBIA
2026-11-15 16:39:45,219 WARNING urbia.inferencia inferencia entrada=originales/2026/11/aaaaaaaaaaaaaaaaaaaaaaaaaa.jpg resultado=error clase=None confianza=None cajas=0 ms=0.2 codigo=IMAGEN_NO_ENCONTRADA
 24  ruta inexistente                           error         -            -     0     0.2  [IMAGEN_NO_ENCONTRADA] La imagen no existe en el almacenamiento

Peticiones: 24 | errores controlados: 4 | el servicio siguió activo: sí | cargas del modelo: 1
Tiempo por imagen (sin errores): mediana 19.0 ms, máximo 43.4 ms
```