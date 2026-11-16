# Servidor del servicio de inferencia

> **Historia de usuario 05.5 — Exponer servicio de inferencia**
> Módulo 05 — Servicio de inferencia de visión artificial · Backend — MVP
> Depende de: 05.4 (contrato gRPC, ver [`contrato-grpc.md`](./contrato-grpc.md)) y 05.3 (servicio, ver [`servicio-inferencia.md`](./servicio-inferencia.md))

*Como integrante del equipo de backend, quiero exponer el servicio de inferencia, para permitir que el backend solicite detecciones sin que el servicio Python gestione la lógica de negocio.*

## 1. Qué hace y qué no

| Sí | No |
|---|---|
| Escucha gRPC (`urbia.inferencia.v1`) y `GET /health` | Guardar en la base de datos (ni siquiera tiene driver de PostgreSQL) |
| Valida la solicitud y la ruta **antes** de leer el disco | Cambiar el estado del reporte o decidir la categoría final |
| Ejecuta la inferencia con límite de tiempo y normaliza el resultado | Escribir, mover o borrar fotos (`STORAGE_DIR` se lee en modo solo lectura) |
| Registra una línea de log por petición | Reintentar por su cuenta: eso lo decide Go con `Reintentable()` |
| Se apaga de forma ordenada (`SIGTERM`, Ctrl+C) | |

Go recibe `DetectarResponse`, decide qué guardar en `deteccion` y si el reporte cambia de estado.

## 2. Arquitectura

```
                                proceso Python (scripts/servidor_grpc.py)
                ┌──────────────────────────────────────────────────────────────────┐
Go monitor ───▶ │ GET /health  (salud.py, hilo aparte)  200 listo / 503 iniciando…  │
INFERENCE_URL   │                                                                  │
                │ gRPC  (contrato_grpc.py)                                         │
Go cliente ───▶ │  InterceptorRegistro ── log por petición, excepción → INTERNAL   │
INFERENCE_GRPC  │   └ InferenciaServicer.Detectar                                  │
_ADDR           │       1. validar_solicitud   id, ids, ruta (patrón de Go)        │
                │       2. límite de tiempo    min(deadline de Go, 8 s)            │
                │       3. ServicioInferencia.inferir_archivo   (05.3, modelo 1 vez)│
                │       4. normalizar          normalizacion.py                    │
                │       5. a_respuesta         DetectarResponse                    │
                └──────────────────────────────────────────────────────────────────┘
```

| Archivo | Responsabilidad |
|---|---|
| `urbia_vision/configuracion_servidor.py` | Variables de entorno del proceso (puertos, hilos, timeout, log) |
| `urbia_vision/contrato_grpc.py` | Validación, límite de tiempo, normalización y traducción del contrato |
| `urbia_vision/normalizacion.py` | Revisa que el resultado cumpla el contrato antes de enviarlo |
| `urbia_vision/registro.py` | Formato de log, interceptor por petición y métricas |
| `urbia_vision/salud.py` | `GET /health` con la biblioteca estándar |
| `urbia_vision/servidor.py` | Ciclo de vida: health → modelo → gRPC → apagado |
| `scripts/servidor_grpc.py` | Punto de entrada y códigos de salida |

## 3. Ciclo de vida

| # | Paso | Health |
|---|---|---|
| 1 | Abre `GET /health` | `503 iniciando` |
| 2 | Carga el modelo **una vez** (SHA-256, clases, calentamiento) | `503 iniciando` |
| 3 | Abre el puerto gRPC | — |
| 4 | Listo para atender | `200 listo` |
| 5 | `SIGTERM` / Ctrl+C / Ctrl+Break | `503 apagando` de inmediato |
| 6 | Rechaza peticiones nuevas (`UNAVAILABLE` → Go: `SERVICIO_NO_DISPONIBLE`) y espera hasta `URBIA_APAGADO_GRACIA` a las que están en curso | `503 apagando` |
| 7 | Cierra gRPC y health; sale con código 0 | — |

Códigos de salida: `0` detenido normalmente · `2` el modelo no cargó (`MODELO_NO_DISPONIBLE` / `MODELO_INVALIDO`) · `3` configuración inválida o puerto ocupado. Si el modelo no carga, el health también se cierra: no queda nada escuchando.

## 4. Validación antes de procesar

`validar_solicitud()` revisa todo lo que se puede **sin tocar el disco ni el modelo**:

| Regla | Código |
|---|---|
| `id_solicitud` de máximo 128 caracteres y sin caracteres de control (evita inyectar líneas falsas en el log). El `X-Request-ID` de Go (16 hex o ≤ 64 caracteres) siempre pasa | `SOLICITUD_INVALIDA` |
| `id_reporte` y `id_archivo` > 0 | `SOLICITUD_INVALIDA` |
| `ruta_archivo` obligatoria y `max_cajas` ≤ 100 | `SOLICITUD_INVALIDA` |
| `ruta_archivo` ≤ 255 caracteres y con el patrón de Go `^(originales\|procesadas)/AAAA/MM/<26 base32>.(jpg\|png\|webp)$` | `RUTA_INVALIDA` |

Después, el lector de 05.3 vuelve a validar la ruta y además que, ya resuelta, no salga de `STORAGE_DIR` (symlinks, junctions).

## 5. Normalización del resultado

Antes de responder, `normalizar()` garantiza que Go reciba algo que sabe guardar:

- `resultado` solo `detectado` o `no_detectado`.
- `detectado`: clase de `clases.py` y confianza entre el umbral y 1, con 4 decimales.
- `no_detectado`: clase y confianza vacías y ninguna caja sobre el umbral.
- Cajas: clase conocida, confianza finita ≥ umbral, coordenadas recortadas a la foto con 1 decimal, sin cajas de menos de 1 px, de mayor a menor confianza.

Si algo no cuadra (NaN, clase desconocida, `detectado` bajo el umbral…) es un fallo del modelo o del código: se responde `INTERNAL` + `FALLO_INFERENCIA` y se registra en el log.

## 6. Timeout, excepciones y logging

**Límite de tiempo.** Cada inferencia tiene como máximo `min(tiempo que le queda a Go, URBIA_TIMEOUT_INFERENCIA)`. El valor por defecto (8 s) es menor que `INFERENCE_GRPC_TIMEOUT` de Go (10 s), así Go recibe `TIEMPO_AGOTADO` con trailer en vez de cortar la llamada sin saber qué pasó. Si a la petición le quedan menos de 0.05 s al empezar, no se infiere.

> PyTorch no permite interrumpir una inferencia que ya empezó. Si se agota el tiempo, la respuesta sale de inmediato y la inferencia termina en segundo plano y se descarta. Si aún estaba en cola, se cancela.

**Excepciones.** Ninguna petición tumba el servidor:

| Situación | Respuesta |
|---|---|
| Error de la foto (`RUTA_INVALIDA`, `IMAGEN_INVALIDA`, …) | Código gRPC + trailer, nivel `WARNING` |
| Error del modelo (por ejemplo, CUDA sin memoria) | `INTERNAL` + `FALLO_INFERENCIA`, nivel `ERROR` |
| Excepción inesperada en el código | El interceptor registra la traza y responde `INTERNAL` + `FALLO_INFERENCIA` sin filtrar detalles internos |

**Logging.** Una línea por petición, con el mismo `id_solicitud` que el `X-Request-ID` de Go:

```
INFO urbia.inferencia grpc metodo=Detectar id_solicitud=9f86d081884c7d65 id_reporte=7 id_archivo=42 codigo=OK urbia_codigo=- ms=183.4 peer=ipv4:127.0.0.1:56088
WARNING urbia.inferencia grpc metodo=Detectar id_solicitud=cli-1 id_reporte=1 id_archivo=1 codigo=INVALID_ARGUMENT urbia_codigo=RUTA_INVALIDA ms=0.3 peer=…
```

Nivel: `INFO` si salió bien, `WARNING` si el error es de la petición (`INVALID_ARGUMENT`, `NOT_FOUND`, `CANCELLED`), `ERROR` si es del servicio. Los valores se limpian (sin espacios ni saltos de línea, máximo 64 caracteres). Las consultas a `/health` solo se registran en `DEBUG`.

## 7. `GET /health`

```json
{
  "estado": "listo",
  "servicio": "urbia-inferencia",
  "version_contrato": "1.0.0",
  "segundos_activo": 42.3,
  "modelo": {"nombre": "urbia-yolo26s", "version": "1.0.0", "umbral_confianza": 0.25, "dispositivo": "0"},
  "inferencias": {"peticiones": 12, "errores": 1},
  "grpc": {"peticiones": 13, "errores": 2, "por_codigo": {"OK": 11, "INVALID_ARGUMENT": 2}}
}
```

`200` solo con `estado = listo`; `503` con `iniciando` o `apagando`. Otra ruta → `404`; otro método → `405`. No expone rutas de fotos, SHA-256, variables de entorno ni trazas. Es lo que espera el `InferenciaMonitor` de Go (`docs/backend/salud.md`).

## 8. Configuración

| Variable | Por defecto | En Docker | Lado Go |
|---|---|---|---|
| `URBIA_GRPC_DIRECCION` | `127.0.0.1:50051` | `0.0.0.0:50051` | `INFERENCE_GRPC_ADDR` |
| `URBIA_SALUD_DIRECCION` | `127.0.0.1:8000` | `0.0.0.0:8000` | `INFERENCE_URL` |
| `URBIA_GRPC_HILOS` | `4` (1–32) | | |
| `URBIA_TIMEOUT_INFERENCIA` | `8` s (0.5–60) | | menor que `INFERENCE_GRPC_TIMEOUT` (10 s) |
| `URBIA_APAGADO_GRACIA` | `10` s (0–120) | menor que el `stop_grace_period` de Compose | |
| `URBIA_LOG_NIVEL` | `INFO` | | |

Más las de 05.3 (`URBIA_MODELO_CONFIG`, `STORAGE_DIR`, `URBIA_DEVICE`, …). `--grpc` y `--salud` en la línea de comandos tienen prioridad sobre el entorno. El modelo usa un candado (Ultralytics no es *thread-safe*), así que más hilos no aceleran la inferencia: solo permiten aceptar y validar peticiones mientras otra se procesa.

## 9. Cómo probarlo

```powershell
cd inference-python
python -m pytest -v                                   # todas, incluidas las de esta HU
python -m pytest tests/test_servidor.py -v            # solo el servidor

# Terminal 1
python -m scripts.servidor_grpc
# Terminal 2
curl.exe -s http://localhost:8000/health
# Terminal 3 (backend-go/)
go run ./cmd/probar-inferencia -info
go run ./cmd/probar-inferencia -ruta originales/2026/11/<26 caracteres>.jpg
go run ./cmd/probar-inferencia -ruta ../../etc/passwd
```

`tests/test_servidor.py` está organizado por subtarea: servidor y health, validación, normalización, respuesta de punta a punta (lector + preprocesamiento + decisión reales con un YOLO falso), timeout/excepciones/logs y que el servicio **no escribe** (el contenido y la fecha de `STORAGE_DIR` no cambian, no se importa ningún driver de base de datos y el código no tiene operaciones de escritura).

## 10. Evidencia

```
2026-11-15 20:52:56,705 INFO urbia.inferencia configuración grpc=127.0.0.1:50051 salud=127.0.0.1:8000 hilos=4 timeout_inferencia=8.0 gracia_apagado=10.0 nivel_log=INFO
2026-11-15 20:52:56,721 INFO urbia.inferencia health escuchando direccion=127.0.0.1:8000 estado=iniciando
2026-11-15 20:52:56,721 INFO urbia.inferencia iniciando servicio storage_dir=C:\Users\junio\Downloads\URBIA\inference-python\..\backend-go\storage modelo_config=C:\Users\junio\Downloads\URBIA\inference-python\config\modelo.json
2026-11-15 20:53:01,125 INFO urbia.inferencia modelo cargado nombre=urbia-yolo26s version=1.0.0 umbral=0.25 imgsz=640 dispositivo=0 sha256=9102fa7191fa segundos=4.40
2026-11-15 20:53:01,146 INFO urbia.inferencia servidor listo grpc=127.0.0.1:50051 salud=127.0.0.1:8000 contrato=v1.0.0 modelo=urbia-yolo26s version=1.0.0 dispositivo=0 hilos=4 timeout_inferencia=8.0s
2026-11-15 20:54:28,838 INFO urbia.inferencia grpc metodo=ObtenerInfo id_solicitud=- id_reporte=- id_archivo=- codigo=OK urbia_codigo=- ms=0.1 peer=ipv4:127.0.0.1:52154
2026-11-15 20:55:48,596 INFO urbia.inferencia inferencia entrada=originales/2026/11/2abnyn5lf546va6tynxusqezq5.jpg resultado=no_detectado clase=None confianza=None cajas=0 ms=182.4
2026-11-15 20:55:48,596 INFO urbia.inferencia grpc metodo=Detectar id_solicitud=cli-1794797748374880600 id_reporte=1 id_archivo=1 codigo=OK urbia_codigo=- ms=183.7 peer=ipv4:127.0.0.1:52316
2026-11-15 20:56:04,255 WARNING urbia.inferencia grpc metodo=Detectar id_solicitud=cli-1794797764227189900 id_reporte=1 id_archivo=1 codigo=INVALID_ARGUMENT urbia_codigo=RUTA_INVALIDA ms=0.1 peer=ipv4:127.0.0.1:52384
2026-11-15 20:56:26,457 WARNING urbia.inferencia inferencia entrada=originales/2026/11/cccccccccccccccccccccccccc.jpg resultado=error clase=None confianza=None cajas=0 ms=0.3 codigo=IMAGEN_NO_ENCONTRADA
2026-11-15 20:56:26,457 WARNING urbia.inferencia grpc metodo=Detectar id_solicitud=cli-1794797786422597000 id_reporte=1 id_archivo=1 codigo=NOT_FOUND urbia_codigo=IMAGEN_NO_ENCONTRADA ms=0.6 peer=ipv4:127.0.0.1:52421
```

```
HTTP/1.0 200 OK
Server: urbia-inferencia 
Date: Mon, 16 Nov 2026 02:53:25 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 317
Cache-Control: no-store

{"estado": "listo", "servicio": "urbia-inferencia", "version_contrato": "1.0.0", "segundos_activo": 28.3, "modelo": {"nombre": "urbia-yolo26s", "version": "1.0.0", "umbral_confianza": 0.25, "dispositivo": "0"}, "inferencias": {"peticiones": 0, "errores": 0}, "grpc": {"peticiones": 0, "errores": 0, "por_codigo": {}}}
```

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
  "peticiones": 0,
  "errores": 0
}
```