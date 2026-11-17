# Modelo de detección de URBIA

> **Historia de usuario 05.2 — Seleccionar, entrenar o ajustar el modelo YOLO**
> Módulo 05 — Servicio de inferencia de visión artificial · Backend — MVP
> Depende de: 05.1 (dataset y clases, ver [`dataset.md`](./dataset.md))

## 1. Selección de la variante

**Decisión:** ajuste fino (*fine-tuning*) de **YOLO26** preentrenado en COCO, con la escala elegida según el hardware.

¿Por qué ajustar y no usar el modelo base? El YOLO26 de COCO trae 80 clases (persona, auto, botella…), pero **ninguna es `bache` ni `basura`**. Por eso hay que entrenarlo con nuestro dataset. Partir de pesos preentrenados (en lugar de entrenar desde cero) aprovecha lo que el modelo ya sabe de bordes, texturas y formas, y necesita muchas menos imágenes y horas.

| Variante | mAP50-95 en COCO | CPU ONNX (ms) | GPU T4 TensorRT (ms) | Parámetros | Cuándo se usa en URBIA |
|---|---|---|---|---|---|
| `yolo26n` | 40.9 | 38.9 | 1.7 | 2.4 M | Sin GPU, o GPU con menos de 4 GB |
| `yolo26s` | 48.6 | 87.2 | 2.5 | 9.5 M | GPU NVIDIA con 4 GB o más (**elegida**) |

Las escalas `m`, `l` y `x` se descartan: son más lentas en CPU y el servicio de la demo puede correr sin GPU.

**Equipo de entrenamiento:** Acer Nitro AN515-58, Intel Core i5-12500H, 16 GB de RAM, **NVIDIA GeForce RTX 3050 Ti Laptop (4 GB de VRAM)**. Con precisión mixta (AMP) y batch 8, `yolo26s` debería caber en 4 GB; la prueba de humo (sección 3) lo confirma antes del entrenamiento completo. Para el servicio, la inferencia en CPU de una foto por reporte (del orden de 100 ms con `yolo26s`) es suficiente.

| VRAM | Variante | Batch |
|---|---|---|
| 6 GB o más | `yolo26s` | automático (-1) |
| 4 GB (este equipo) | `yolo26s` | 8 (4 si aparece *CUDA out of memory*) |
| Menos de 4 GB | `yolo26n` | 8 |
| Sin GPU | `yolo26n` en CPU | 8 (mejor usar Colab) |

La regla está en `urbia_vision/hardware.py`. Para ver qué toca en tu equipo:

```powershell
python -m scripts.info_hardware
```

### 1.1 Licencia

- Ultralytics y sus modelos (incluido YOLO26) se publican bajo **AGPL-3.0**. Según Ultralytics, **los modelos que se entrenan con su código también quedan bajo AGPL-3.0**.
- El uso académico y en proyectos escolares está cubierto por AGPL-3.0. La condición es publicar el código fuente completo, y el repositorio de URBIA ya es público.
- Si URBIA se usara comercialmente sin abrir el código, haría falta la licencia *Enterprise* de Ultralytics o cambiar a un detector con licencia permisiva (Apache-2.0 / MIT).

## 2. Configuración de entrenamiento

Archivo: `inference-python/config/entrenamiento.yaml`

| Parámetro | Valor | Motivo |
|---|---|---|
| `modelo_base` | `auto` | `yolo26s` o `yolo26n` según la VRAM (sección 1) |
| `epochs` / `patience` | 100 / 20 | Tope de 100 épocas; para antes si `val` no mejora en 20 seguidas |
| `imgsz` | 640 | Tamaño estándar con el que se preentrenó YOLO26 |
| `batch` | `auto` | -1 (automático) con 6 GB o más; 8 con menos VRAM o en CPU |
| `seed` / `deterministic` | 42 / `true` | Resultados repetibles |
| `close_mosaic` | 10 | Las últimas 10 épocas entrenan con fotos normales, sin collage |
| `amp` | `true` | Precisión mixta en GPU (se desactiva sola en CPU) |

Ultralytics guarda como `best.pt` la época con mayor **mAP50-95 en `val`**. El split `test` no se usa durante el entrenamiento.

## 3. Cómo entrenar

```powershell
cd inference-python
python -m scripts.entrenar --epochs 1 --fraccion 0.05   # prueba de humo (minutos)
python -m scripts.entrenar                              # entrenamiento completo
```

Resultado: `runs/urbia/entrenamiento/` con `weights/best.pt`, `results.csv`, gráficas de pérdida y `entrenamiento_urbia.json` (hardware, argumentos y duración).

### 3.1 Sin GPU: Google Colab

Entrenar ~6 700 imágenes en CPU tarda demasiado. En Colab (Entorno de ejecución → GPU T4):

```python
!git clone -b feature/modelo-yolo https://github.com/PL290723/URBIA.git
%cd URBIA/inference-python
!pip install -q -r requirements-dev.txt
# Subir datasets_urbia.zip (la carpeta datasets/urbia comprimida) al panel de archivos
!mkdir -p datasets && unzip -q /content/datasets_urbia.zip -d datasets/
!sed -i "s#^path:.*#path: $(pwd)/datasets/urbia#" datasets/urbia/data.yaml
!python -m scripts.validar_dataset
!python -m scripts.entrenar
!python -m scripts.evaluar --pesos runs/urbia/entrenamiento/weights/best.pt
!zip -qr resultado.zip runs/urbia/entrenamiento
```

Descargar `resultado.zip`, descomprimirlo en `inference-python/runs/urbia/` y continuar con la sección 6 en la computadora local.

## 4. Evaluación

```powershell
python -m scripts.evaluar --pesos runs/urbia/entrenamiento/weights/best.pt
```

Se evalúa sobre el split **`test`**: fotos sin aumentar que el modelo nunca vio (ver sección 5.1). Genera `runs/urbia/entrenamiento/evaluacion_test/`:

| Archivo | Contenido |
|---|---|
| `reporte.md` | Tablas listas para pegar en la sección 8 |
| `metricas.json` | Todo lo anterior en formato máquina (lo usa `publicar_modelo`) |
| `ultralytics/confusion_matrix*.png` | Matriz de confusión oficial |
| `ultralytics/Box*_curve.png` | Curvas de precisión, recall, F1 y PR |
| `falsos_positivos/`, `falsos_negativos/`, `confusiones/` | Fotos con cajas dibujadas: **verde = real**, **rojo = predicción** |

**Cómo leer las métricas**

- **Precisión:** de lo que el modelo marcó como `bache`, qué fracción sí era bache. Si es baja, hay falsas alarmas.
- **Recall:** de los baches reales, qué fracción encontró. Si es bajo, se le escapan incidencias.
- **mAP50:** calidad general de la detección con un traslape de caja de al menos 50 %.
- **Matriz de confusión:** las filas son lo que predijo el modelo y las columnas lo real. La fila/columna `fondo` significa "nada": `fondo → bache` es un bache no detectado y `bache → fondo` es una falsa alarma.
- **Exactitud por foto:** aplica la regla de la sección 7 a cada foto y la compara con la categoría correcta. Es lo que verá el usuario de URBIA.

## 5. Inspección de falsos positivos y falsos negativos

Se revisaron a mano las carpetas `falsos_positivos/`, `falsos_negativos/` y `confusiones/` que genera `scripts.evaluar` (verde = etiqueta real, rojo = predicción).

### 5.1 Hallazgo principal: el conjunto de prueba tenía imágenes aumentadas

Casi todos los ejemplos tenían **ruido tipo "sal y pimienta"** y estaban **rotados con bordes negros**. No son fotos reales: son **aumentaciones que Roboflow aplicó al exportar** (3 variantes por foto en su `train/`).

Al revisar `manifiesto_split.csv`:

| Fuente | Fotos distintas | Imágenes en el export | Variantes por foto |
|---|---|---|---|
| pothole | 1 196 | 3 195 | la mayoría con 3 |
| garbage | 1 584 | 4 187 | la mayoría con 3 (algunas más) |

La división aleatoria de la HU 05.1 mantuvo juntas las variantes de cada foto, pero **llevó fotos aumentadas a `val` y `test`**. Consecuencias:

- Las métricas de `test` medían al modelo con fotos con ruido artificial, no con fotos como las que tomará un ciudadano. Eran **pesimistas y poco representativas**.
- `best.pt` se eligió con un `val` también aumentado.
- El dataset tiene unas **2 780 fotos distintas**, no 7 382.

**Acción 1:** volver a preparar el dataset con `--split-original`, que respeta `train/`, `valid/` y `test/` del export. Roboflow solo aumenta `train/`, así que `val` y `test` quedan con fotos reales y sin variantes de las fotos de entrenamiento.

**Hallazgo secundario: fotos repetidas entre splits.** Al validar la nueva división, la comparación por contenido encontró la **misma foto en `train` y en `val`/`test`**: 4 archivos idénticos (el autor del dataset de basura subió la misma foto dos veces) y 20 casi idénticos (la misma foto de banco de imágenes con otro nombre, por ejemplo con el título en francés y en italiano, y cuadros casi iguales de un mismo video de baches). Se revisaron a mano.

**Acción 2:** `preparar_dataset --quitar-duplicados` saca esas 24 imágenes de `val`/`test` y las registra en `config/duplicados_quitados.csv`. El dataset final queda en 6 668 / 346 / 344 (ver `dataset.md`, sección 9.1). Después se reentrena y se reevalúa (sección 8).

### 5.2 Patrones de error del modelo

| Tipo | Patrón observado | Posible acción |
|---|---|---|
| Falso negativo / localización | El modelo detecta el bache grande con una caja **mucho más amplia** que la etiqueta (incluye el pavimento dañado alrededor). Con IoU < 0.5 cuenta a la vez como falso positivo y falso negativo, aunque sí "vio" el bache. | Aceptable para URBIA: lo que importa es la categoría sugerida por foto (exactitud por foto), no la caja exacta. |
| Falso negativo | **Baches pequeños, someros o alargados** (franjas claras con agua o reflejo) no se detectan. Algunas de esas etiquetas son ambiguas: parecen charcos o reflejos más que hundimientos. | Revisar la consistencia del etiquetado; la guía de la 05.1 pide etiquetar solo hundimientos con borde visible. |
| Falso positivo | **Grietas y marcas oscuras alargadas** en el pavimento detectadas como `bache` con **confianza baja (~0.35)**, mientras el bache real sale con 0.84. | Están justo en el borde del umbral. Si al reevaluar con el `test` limpio siguen apareciendo, conviene subir el umbral un paso (0.05). |
| Ruido de la imagen | Marcas de agua o texto en la foto y bordes negros de la rotación. | Se resuelve con el `test` limpio (5.1). |
| Confusión de clase | En 5 de 96 fotos de bache el modelo sugiere `basura`: entre ellas hay escombros que pareciesen restos de basura, sombras de escombros que parecen proyectarse como si fueran desechos, etc. | Es el error más costoso (sugerencia equivocada). El administrador lo corrige; para una v1.1, agregar fotos de baches con escombro etiquetadas solo como `bache`. |
## 6. Pesos y metadatos de versión

```powershell
python -m scripts.publicar_modelo --pesos runs/urbia/entrenamiento/weights/best.pt --metricas runs/urbia/entrenamiento/evaluacion_test/metricas.json --version 1.0.0
```

- Copia los pesos a `inference-python/models/urbia-<variante>-v1.0.0.pt`. **No se sube a git** porque `*.pt` y `models/` están en `.gitignore`.
- Genera `inference-python/config/modelo.json`, que **sí se versiona**. Contiene nombre, versión, SHA-256 de los pesos, clases, `imgsz`, umbral, época mejor, versiones de Ultralytics/torch/Python, SHA-256 del manifiesto del dataset y métricas de `test`.
- Antes de guardar valida que las clases del modelo sean exactamente las de `clases.py`, que la versión sea `X.Y.Z` y que `nombre` y `version` quepan en `deteccion.modelo` (60) y `deteccion.version_modelo` (20).
- Los pesos se distribuyen como **GitHub Release** (`modelo-v1.0.0`). El servicio (05.3) comparará el SHA-256 del archivo descargado contra `modelo.json` antes de cargarlo.

**Versionado:** `MAYOR.MENOR.PARCHE`. Cambiar las clases es MAYOR; reentrenar con más datos o cambiar la variante es MENOR; ajustar solo el umbral es PARCHE.

## 7. Umbral de confianza y comportamiento sin detección concluyente

**Elección del umbral:** `scripts.evaluar` prueba umbrales de 0.05 a 0.95 (pasos de 0.05) sobre `test` y elige el que da **mayor F1 macro**, es decir, el mejor equilibrio entre falsas alarmas e incidencias perdidas, promediando ambas clases por igual. En empate gana el umbral más alto. El valor queda en `modelo.json → umbral_confianza`.

> Si el umbral elegido sale menor a 0.25, el modelo probablemente está poco entrenado: conviene revisar las curvas y los ejemplos antes de publicarlo.

**Regla de decisión por foto** (`urbia_vision/decision.py`), con los mismos valores que la tabla `deteccion`:

| Situación | `resultado` | `clase_detectada` | `confianza` | Qué pasa con el reporte |
|---|---|---|---|---|
| Al menos una caja con confianza ≥ umbral | `detectado` | clase de la caja más confiable | su confianza (4 decimales) | Se sugiere esa categoría; el administrador la confirma |
| Ninguna caja alcanza el umbral | `no_detectado` | `NULL` | `NULL` | Queda sin categoría sugerida (`id_categoria` en `NULL`) y el administrador la asigna manualmente |
| Imagen inválida o fallo del modelo | `error` | `NULL` | `NULL` | Igual que `no_detectado`; se registra el error (05.3 / 05.5) |

El modelo **nunca rechaza ni descarta un reporte**: solo sugiere. Una foto de una incidencia que no es bache ni basura (por ejemplo, una luminaria) debe dar `no_detectado`. Ese es el comportamiento correcto.

## 8. Resultados

| Dato | Valor |
|---|---|
| Variante | `yolo26s` (ajuste fino desde COCO) |
| Hardware | NVIDIA GeForce RTX 3050 Ti Laptop (4 GB), batch 8, AMP |
| Épocas entrenadas / época mejor | 64 / 44 (early stopping con `patience` 20) |
| Duración | 184 min (3.1 h) |
| mAP50 / mAP50-95 (`val`, 346 fotos) | 0.822 / 0.537 |
| mAP50 / mAP50-95 (`test`, 344 fotos) | **0.791 / 0.485** |
| Umbral de confianza | 0.25 (máximo F1 macro 0.750) |
| Exactitud de la sugerencia por foto (`test`) | **96.5 %** (332 de 344) |
| Pesos | `models/urbia-yolo26s-v1.0.0.pt` \ SHA-256 `9102fa7191fa950537aaa4f4b12df1cef09aecc6a419b03927ab50174e335ef7` |

**Curvas de entrenamiento.** Las pérdidas de entrenamiento bajan de forma continua; las de validación se aplanan hacia la época 40–45 y después suben ligeramente (inicio de sobreajuste). El early stopping detuvo el entrenamiento en la época 64 y conservó la época 44 como `best.pt`.

**Métricas por clase en `test` (Ultralytics):**

| Clase | Precisión | Recall | mAP50 | mAP50-95 |
|---|---|---|---|---|
| todas | 0.840 | 0.705 | 0.791 | 0.485 |
| bache | 0.833 | 0.652 | 0.752 | 0.468 |
| basura | 0.847 | 0.757 | 0.829 | 0.502 |

**Al umbral 0.25 (cajas con IoU ≥ 0.5):**

| Clase | TP | FP | FN | Precisión | Recall | F1 |
|---|---|---|---|---|---|---|
| bache | 110 | 29 | 66 | 0.791 | 0.625 | 0.698 |
| basura | 206 | 45 | 57 | 0.821 | 0.783 | 0.802 |

| Cajas: predicho \ real | bache | basura | fondo |
|---|---|---|---|
| **bache** | 110 | 0 | 29 |
| **basura** | 6 | 206 | 39 |
| **fondo** | 60 | 57 | 0 |

**Sugerencia de categoría por foto** (regla de la sección 7):

| Fotos: predicho \ real | bache | basura | fondo |
|---|---|---|---|
| **bache** | 86 | 0 | 0 |
| **basura** | 5 | 223 | 1 |
| **fondo** | 5 | 1 | 23 |

**Lectura de los resultados**

- El modelo generaliza bien: la baja de `val` a `test` es pequeña (mAP50 0.822 → 0.791).
- A nivel de caja el recall es moderado (se le escapan baches pequeños o someros), pero **a nivel de foto acierta la categoría en el 96.5 %**: basta con encontrar una incidencia por foto para sugerir bien.
- `bache` es la clase más difícil: 90 % de acierto por foto contra 99.6 % de `basura`.
- El error más costoso son las **5 fotos de bache sugeridas como `basura`** (ver sección 5.2). El administrador confirma la categoría, así que no se pierde ningún reporte.
- Estas métricas son sobre fotos de otros países; el conjunto local de Celaya (`dataset.md`, sección 5.1) queda pendiente para medir el desempeño real en la ciudad.