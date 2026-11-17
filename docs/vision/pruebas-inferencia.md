# Pruebas del servicio de inferencia (HU 09.3)

Verifican que el modelo procesa imágenes y que maneja bien los resultados inciertos.
Hay dos niveles:

| Nivel | Dónde | Qué usa | Cuándo correrlo |
|---|---|---|---|
| Automáticas (pytest) | `inference-python/tests/test_casos_inferencia.py` | gRPC y health **reales**, `DetectorUrbia.cargar` real, STORAGE_DIR temporal; solo YOLO es falso | Siempre, sin GPU ni pesos |
| Manuales (Bruno) | colección `local`, carpeta `09.3 Servicio de inferencia` | API Go + Python + PostgreSQL con el **modelo real** y fotos reales | Antes de una demostración |

## Cómo correr las automáticas

```powershell
cd inference-python            # con tu entorno de Python activado
python -m pytest tests/test_casos_inferencia.py -v      # solo las de esta HU
python -m pytest                                        # todo el servicio
```

## Casos cubiertos

| Subtarea | Casos | Resultado esperado |
|---|---|---|
| Imagen válida | JPEG, PNG, PNG con transparencia, WebP; foto de 2560 px; foto chica | `RESULTADO_DETECTADO` o `NO_DETECTADO`, ancho/alto originales, modelo y versión, cajas en píxeles de la foto original; la foto se reduce a 1280 px y nunca se agranda |
| Archivo corrupto / no imagen | texto `.jpg`, HTML `.png`, GIF, JPEG truncado, PNG sin datos, 100×100 px | `INVALID_ARGUMENT` + `IMAGEN_INVALIDA`; **nunca llega al modelo**; el servicio sigue atendiendo y el health cuenta los errores |
| | archivo vacío, inexistente, ruta con `../` | `NOT_FOUND IMAGEN_NO_ENCONTRADA` / `INVALID_ARGUMENT` |
| Detección concluyente | una caja de bache; varias clases; confianza con 9 decimales; `max_cajas`, `omitir_cajas` | Gana la caja más confiable; confianza a 4 decimales (`NUMERIC(5,4)`); cajas ordenadas |
| Ausencia de detección | el modelo no devuelve cajas | `RESULTADO_NO_DETECTADO`, clase vacía (NULL), confianza 0, sin cajas, **con** modelo y versión; no cuenta como error |
| Confianza bajo el umbral | 0.24, 0.2499 y varias bajas → no detectado; 0.25 y 0.2501 → detectado; modelo que "ignora" `conf` | El umbral (0.25) es inclusivo y la regla de decisión filtra aunque YOLO devuelva de más; el umbral sale de `modelo.json` |
| Modelo o pesos ausentes | sin pesos, sin `modelo.json`, SHA-256 distinto, JSON roto, campos faltantes, umbral fuera de rango, otras clases, pesos que Ultralytics no abre | `ModeloNoDisponible` / `ModeloInvalido`; el servidor **no queda escuchando** (Go guarda el reporte con `no_disponible`); al restaurar los pesos vuelve a cargar |
| Registrar evaluación | `config/modelo.json` y este documento | Precisión, recall y F1 recalculados desde TP/FP/FN; F1 macro; métricas en [0, 1]; `modelo.md` muestra el SHA-256 y las cifras reales |

## Registro de la evaluación del modelo

Modelo evaluado: **urbia-yolo26s v1.0.0** · pesos `models/urbia-yolo26s-v1.0.0.pt` ·
SHA-256 `9102fa7191fa950537aaa4f4b12df1cef09aecc6a419b03927ab50174e335ef7`.

Conjunto: split `test` (344 fotos que el modelo nunca vio), manifiesto `config/manifiesto_split.csv`.
Se generó con `python -m scripts.evaluar` y quedó publicado en `config/modelo.json → metricas_test`.

**Métricas de Ultralytics (IoU 0.5 y 0.5–0.95):**

| Clase | Precisión | Recall | mAP50 | mAP50-95 |
|---|---|---|---|---|
| todas | 0.840 | 0.705 | 0.791 | 0.485 |
| bache | 0.833 | 0.652 | 0.752 | 0.468 |
| basura | 0.847 | 0.757 | 0.829 | 0.502 |

**Al umbral de operación 0.25** (el que usa el servicio; elegido por máximo F1 macro):

| Clase | TP | FP | FN | Precisión | Recall | F1 |
|---|---|---|---|---|---|---|
| bache | 110 | 29 | 66 | 0.791 | 0.625 | 0.698 |
| basura | 206 | 45 | 57 | 0.821 | 0.783 | 0.802 |

- F1 macro: **0.750**.
- Exactitud de la categoría sugerida por foto: **96.5 %** (332 de 344).
- Resultado incierto: si ninguna caja alcanza 0.25, el servicio responde `no_detectado` y el
  reporte queda **sin categoría sugerida**; el ciudadano o el administrador la eligen.

Al publicar un modelo nuevo (`scripts.publicar_modelo`), actualizar este registro y
`modelo.md`. La prueba `test_registro_de_pruebas_de_inferencia` falla si los TP/FP/FN de
aquí no coinciden con `modelo.json`.

## Pruebas con fotos reales (Bruno)

Llenar después de correr la carpeta `09.3 Servicio de inferencia`:

## Pruebas con fotos reales (Bruno)

Resultados de la carpeta `09.3 Servicio de inferencia` con urbia-yolo26s v1.0.0:

| Foto | Esperado | Resultado | Confianza | Fecha |
|---|---|---|---|---|
| `bache.jpeg` | sugerida · bache | 201 · sugerida · bache (reporte 89, detección 76) | 0.5409 | 2026-11-17 |
| `basura_tres.jpeg` | sugerida · basura | 201 · sugerida · basura (reporte 90, detección 77) | 0.8585 | 2026-11-17 |
| `limpia.jpg` | sin_deteccion | 201 · sin_deteccion, sin categoría; Python respondió con urbia-yolo26s 1.0.0 (reporte 91, detección 78) | — | 2026-11-17 |
| texto disfrazado de `.jpg` | 415, no llega a Python | 415 · FORMATO_NO_PERMITIDO | — | 2026-11-17 |
| JPEG truncado | 422, no llega a Python | 422 · IMAGEN_INVALIDA | — | 2026-11-17 |
| Python sin modelo | no arranca; reporte `no_disponible` | API `degradado` · inferencia `caido` · PostgreSQL `activo`; el reporte se guarda con `no_disponible` | — | 2026-11-17 |

Las dos detecciones superan el umbral de 0.25. El bache sale con una confianza moderada
(0.54), lo que concuerda con la evaluación: `bache` es la clase más difícil (recall 0.625).