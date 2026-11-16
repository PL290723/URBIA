# Contrato gRPC entre Go y Python

> **Historia de usuario 05.4 — Definir contrato gRPC entre Go y Python**
> Módulo 05 — Servicio de inferencia de visión artificial · Backend — MVP
> Depende de: 05.3 (servicio de inferencia, ver [`servicio-inferencia.md`](./servicio-inferencia.md))

| | |
|---|---|
| Archivo | [`proto/urbia/inferencia/v1/inferencia.proto`](../../proto/urbia/inferencia/v1/inferencia.proto) |
| Paquete | `urbia.inferencia.v1` |
| Versión | **1.0.0** |
| Servicio | `InferenciaService` |
| Transporte | gRPC sin TLS, solo en la red interna (en Docker el puerto no se publica) |
| Dirección | Python escucha en `URBIA_GRPC_DIRECCION` (por defecto `127.0.0.1:50051`); Go llama a `INFERENCE_GRPC_ADDR` |

## 1. Flujo

```
API Go                                             Servicio Python
──────                                             ───────────────
reporte + archivo guardados (04.1)
ClienteDeteccion.Detectar(ctx, solicitud) ──gRPC──▶ InferenciaServicer.Detectar
  timeout INFERENCE_GRPC_TIMEOUT (10 s)              valida la solicitud
                                                     ServicioInferencia.inferir_archivo (05.3)
                          ◀── DetectarResponse ───── resultado DETECTADO / NO_DETECTADO
                          ◀── error + trailer ────── código gRPC + "urbia-codigo"
Go decide qué guardar en `deteccion` (Python nunca toca la BD ni el reporte)
```

## 2. Operaciones

| RPC | Para qué |
|---|---|
| `Detectar(DetectarRequest) → DetectarResponse` | Detecta incidencias en una foto ya guardada en `STORAGE_DIR` |
| `ObtenerInfo(ObtenerInfoRequest) → ObtenerInfoResponse` | Versión del contrato, modelo cargado, umbral, clases, dispositivo, SHA-256 y contadores |

## 3. Solicitud: `DetectarRequest`

| # | Campo | Tipo | Obligatorio | De dónde sale en Go |
|---|---|---|---|---|
| 1 | `id_solicitud` | string | No | `X-Request-ID` de la petición HTTP; se devuelve igual en la respuesta para seguirla en los logs |
| 2 | `id_reporte` | int64 | Sí, > 0 | `reporte.id_reporte` (Python solo lo usa en logs) |
| 3 | `id_archivo` | int64 | Sí, > 0 | `archivo.id_archivo` (solo logs) |
| 4 | `ruta_archivo` | string | Sí | `archivo.ruta`, relativa a `STORAGE_DIR` |
| 5 | `opciones.omitir_cajas` | bool | No | `true` = solo el resultado, sin cajas |
| 5 | `opciones.max_cajas` | uint32 | No | 0 = todas; máximo 100; las de mayor confianza |

El **umbral de confianza no es una opción**: lo fija el modelo publicado (`config/modelo.json`, HU 05.2), para que todas las sugerencias se decidan igual.

## 4. Respuesta: `DetectarResponse`

| # | Campo | Con `DETECTADO` | Con `NO_DETECTADO` | Columna de `deteccion` |
|---|---|---|---|---|
| 1 | `id_solicitud` | eco | eco | — |
| 2 | `resultado` | `RESULTADO_DETECTADO` | `RESULTADO_NO_DETECTADO` | `resultado` |
| 3 | `clase_detectada` | `bache` / `basura` | `""` → Go: `nil` | `clase_detectada` (NULL) |
| 4 | `confianza` | 0..1 | `0` → Go: `nil` | `confianza` (NULL) |
| 5 | `cajas` | ordenadas por confianza | vacío | — |
| 6 | `modelo` | nombre, versión, umbral | igual | `modelo`, `version_modelo` |
| 7–8 | `ancho`, `alto` | px de la foto original | igual | — |
| 9 | `milisegundos` | tiempo dentro de Python | igual | — |

- Las coordenadas de las cajas están en **píxeles de la foto original**, con la orientación ya aplicada.
- `RESULTADO_NO_ESPECIFICADO` (0) **nunca se envía**. Si Go lo recibe, responde `CONTRATO_INVALIDO`: los dos servicios tienen versiones distintas del `.proto`.
- protobuf usa `float32`. Go redondea la confianza y el umbral a 4 decimales (`NUMERIC(5,4)` en la BD) y los píxeles y milisegundos a 1 decimal.

## 5. Errores

Un error **no** viaja dentro de `DetectarResponse`: se responde con un **código de estado gRPC**, y el código de URBIA va en el metadato final (*trailer*) `urbia-codigo`. Así cualquier cliente gRPC entiende el error, y Go además sabe exactamente qué pasó.

| `urbia-codigo` | Código gRPC | Lo genera | Causa | ¿Reintentar? |
|---|---|---|---|---|
| `SOLICITUD_INVALIDA` | `INVALID_ARGUMENT` | Python | Falta `id_reporte`, `id_archivo` o `ruta_archivo`, o `max_cajas` > 100 | No |
| `RUTA_INVALIDA` | `INVALID_ARGUMENT` | Python | La ruta no cumple el patrón de Go o sale de `STORAGE_DIR` | No |
| `IMAGEN_INVALIDA` | `INVALID_ARGUMENT` | Python | Formato, tamaño o dimensiones no válidos; archivo dañado | No |
| `IMAGEN_NO_ENCONTRADA` | `NOT_FOUND` | Python | La foto no existe o está vacía | No |
| `FALLO_INFERENCIA` | `INTERNAL` | Python | El modelo falló con esa imagen (p. ej. sin memoria en la GPU) | Sí, una vez |
| `MODELO_NO_DISPONIBLE` / `MODELO_INVALIDO` | `UNAVAILABLE` | Python | El modelo no está cargado | Sí, más tarde |
| `TIEMPO_AGOTADO` | `DEADLINE_EXCEEDED` | Python o Go | Se acabó el tiempo de la llamada | Sí, más tarde |
| `SERVICIO_NO_DISPONIBLE` | `UNAVAILABLE` | Go | No hay conexión con Python (apagado, puerto equivocado) | Sí, más tarde |
| `CANCELADO` | `CANCELED` | Go | El contexto se canceló (p. ej. el cliente HTTP cerró la conexión) | No |
| `CONTRATO_INVALIDO` | `UNKNOWN` / `UNIMPLEMENTED` | Go | Respuesta que no cumple el contrato o RPC inexistente | No: hay que alinear versiones |
| `ERROR_DESCONOCIDO` | cualquier otro | Go | — | No |

En Go, `ErrorInferencia.Reintentable()` devuelve `true` para `UNAVAILABLE`, `DEADLINE_EXCEEDED` e `INTERNAL`. Un error de una imagen **no tumba el servidor**: la siguiente solicitud se atiende normal (lo prueban los tests de ambos lados).

## 6. Tiempos límite (timeouts)

| Dónde | Valor | Qué hace |
|---|---|---|
| Go, por llamada | `INFERENCE_GRPC_TIMEOUT` = **10 s** | `context.WithTimeout` en cada `Detectar` / `Info`. Una inferencia tarda ~0.2–0.7 s en CPU; 10 s cubre fotos grandes y varias peticiones en cola (el modelo atiende una a la vez) |
| Go, contexto de la petición HTTP | el que traiga | Si el cliente HTTP se va, la llamada se cancela (`CANCELADO`) |
| Python, al empezar | margen de 0.05 s | Si a la solicitud ya no le queda tiempo, responde `TIEMPO_AGOTADO` **sin** inferir |
| Conexión | sin espera | Go no espera a que Python levante (`WaitForReady` desactivado): si no hay servicio, falla en cuanto el sistema rechaza la conexión (al instante en Linux/Docker, ~2 s en Windows)|

El cliente de Go se crea una vez al iniciar la API. Abre la conexión en la primera llamada y se reconecta solo si Python se reinicia.

## 7. Código generado

El código de ambos lados se genera del `.proto` y **se versiona en git**, para que nadie necesite `protoc` solo para compilar:

| Lado | Archivos |
|---|---|
| Python | `inference-python/urbia/inferencia/v1/inferencia_pb2.py`, `inferencia_pb2.pyi`, `inferencia_pb2_grpc.py` |
| Go | `backend-go/internal/gen/inferenciav1/inferencia.pb.go`, `inferencia_grpc.pb.go` |

Esos archivos **no se editan a mano**. Para regenerarlos (desde `inference-python/`):

```powershell
python -m scripts.generar_contrato              # genera Go y Python
python -m scripts.generar_contrato --verificar  # falla si el código generado no coincide con el .proto
```

El script usa el `protoc` que trae `grpcio-tools` (no hay que instalar `protoc`) y los plugins de Go:

```powershell
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
```

## 8. Versionado

- La versión del contrato aparece en el encabezado del `.proto`, en `VERSION_CONTRATO` (Python), en `VersionContrato` (Go) y en `ObtenerInfo.version_contrato`.
- **Cambio compatible** (agregar un campo, un valor de enum o una RPC) → `1.x.0`, en el mismo paquete `v1`. Los clientes viejos ignoran lo que no conocen.
- **Nunca** se cambia el número ni el tipo de un campo existente. Un campo que se deja de usar se marca `reserved`.
- **Cambio incompatible** → paquete nuevo `urbia.inferencia.v2`, en `proto/urbia/inferencia/v2/`. Durante la transición, el servidor atiende `v1` y `v2`.
- Todo cambio al `.proto` va en el mismo PR que el código regenerado de ambos lados; `--verificar` lo comprueba.

## 9. Cómo probarlo

**Pruebas automáticas.**
- Python (`tests/test_contrato_grpc.py`) levanta un servidor gRPC real en un puerto libre y lo llama con el cliente generado: respuestas, opciones, cada código de error, timeout, peticiones concurrentes y servicio caído.
- Go (`internal/clients/inferencia_grpc_test.go`) usa un servidor en memoria (`bufconn`) que responde como Python: traducción de resultados y errores, `nil` para NULL, timeout, cancelación, servicio caído y contrato inválido.

**Prueba entre lenguajes** (servidor Python real + cliente Go real):

```powershell
# Terminal 1 (inference-python/)
python -m scripts.servidor_grpc

# Terminal 2 (backend-go/)
go run ./cmd/probar-inferencia -info
go run ./cmd/probar-inferencia -ruta originales/2026/11/<26 caracteres>.jpg
go run ./cmd/probar-inferencia -ruta ../../etc/passwd
```

El servidor de esta HU es **mínimo**, solo para validar el contrato. La HU 05.5 lo completa: health check HTTP (`GET /health` en `INFERENCE_URL`), log por petición, apagado ordenado con `SIGTERM` y configuración para Docker.

## 10. Evidencia

```
{
  "version_contrato": "1.0.0",
  "modelo": "urbia-yolo26s",
  "version_modelo": "1.0.0",
  "umbral_confianza": 0.25,
  "clases": [
    "bache",
    "basura"
  ],
  "imgsz": 640,
  "dispositivo": "0",
  "sha256_pesos": "9102fa7191fa950537aaa4f4b12df1cef09aecc6a419b03927ab50174e335ef7",
  "peticiones": 4,
  "errores": 3
}
```