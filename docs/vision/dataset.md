# Dataset y clases de detección de URBIA

> **Historia de usuario 05.1 — Definir dataset y clases de detección**
> Módulo 05 — Servicio de inferencia de visión artificial · Backend — MVP
> Depende de: 01.3 (entorno Python con PyTorch + Ultralytics)

## 1. Clases que detecta el MVP

El MVP detecta **2 clases**. La fuente de verdad en código es
[`inference-python/urbia_vision/clases.py`](../../inference-python/urbia_vision/clases.py).

| id YOLO | Nombre | Categoría en BD (`categoria.nombre`) | Qué SÍ se etiqueta | Qué NO se etiqueta |
|---|---|---|---|---|
| 0 | `bache` | `bache` | Hoyo o hundimiento en el pavimento de la calle, con borde visible. Una caja por bache. | Grietas finas, parches ya reparados, charcos sin hundimiento visible, coladeras. |
| 1 | `basura` | `basura` | Acumulación de basura o tiradero en vía pública: bolsas amontonadas, montón de desechos. Una caja que cubre **todo el montón**. | Un objeto suelto aislado (una lata, una botella), botes de basura cerrados y en su lugar. |

**Por qué solo estas dos**

- Son las incidencias principales del alcance de URBIA (ver README) y las más fáciles de reconocer en una foto tomada con el celular.
- Hay datasets públicos con licencia abierta para ambas (sección 3).
- Las otras 6 categorías de la BD (`alumbrado_publico`, `fuga_agua`, `banqueta_danada`, `senalizacion`, `grafiti`, `arbol_caido`) **no se detectan con IA en el MVP**: el ciudadano o el administrador las elige manualmente. Se pueden agregar después con un id nuevo al final (2, 3, …) sin cambiar los existentes.

**Reglas para mantener el contrato con el backend**

- Los ids no se reordenan nunca. Una clase nueva va al final.
- El nombre de la clase es exactamente `categoria.nombre`. El test `test_clases_existen_como_categoria_en_la_bd` falla si alguien cambia uno sin el otro.
- El backend en Go guarda el nombre en `deteccion.clase_detectada` y lo usa para sugerir `reporte.id_categoria`.

## 2. Datasets revisados

| Dataset | Contenido | Licencia | Decisión |
|---|---|---|---|
| [Pothole Detection (IMACS) — Roboflow Universe](https://universe.roboflow.com/imacs-pothole-detection-wo8mu/pothole-detection-irkz9) | Fotos de calles con baches, 1 clase `pothole` | CC BY 4.0 | **Se usa** para `bache` |
| [garbage_detection — Roboflow Universe](https://universe.roboflow.com/garbage-detection-czeg5/garbage_detection-wvzwv) | Fotos de calle con **acumulaciones** de basura, 1 clase `garbage` | CC BY 4.0 | **Se usa** para `basura` (ver riesgo en 3.2) |
| [RDD2022](https://arxiv.org/abs/2209.08538) | 47 420 fotos de calles de 6 países; grietas (D00, D10, D20) y baches (D40) | CC BY-SA 4.0 | **Opcional** para reforzar `bache` (solo D40) |
| [TACO](https://arxiv.org/abs/2003.06975) | ~1 500 fotos de **objetos sueltos** de basura (botellas, latas, colillas) | CC BY 4.0 | **No se usa** para `basura` (ver sección 7) |

## 3. Licencias

### 3.1 Qué permite CC BY 4.0

Usar, copiar, modificar y redistribuir las imágenes y anotaciones, incluso con fines comerciales,
**siempre que se dé crédito** al autor y se indique la licencia. Para URBIA esto significa:

- Citar cada dataset usado en este documento (sección 8) y en la presentación de la Feria.
- No subir las imágenes al repositorio (ya lo impide `.gitignore` con `datasets/`), solo los scripts y el manifiesto.

### 3.2 Riesgos detectados

| Riesgo | Detalle | Acción |
|---|---|---|
| Origen de las imágenes de `garbage_detection` | Su descripción lo vincula con el proyecto *glasgow-litter*, que tomó fotos de **Google Street View**. Roboflow publica las **anotaciones** como CC BY 4.0, pero las imágenes de Street View tienen sus propios términos de Google. | Uso solo académico y demostrativo (Feria). No redistribuir las imágenes. Si URBIA se usa fuera de la escuela, reemplazar por fotos propias. |
| RDD2022 es CC BY-SA 4.0 | *ShareAlike*: si se publica un dataset derivado, debe ir con la misma licencia. | Si se usa, no se publica el dataset derivado; solo se documenta la atribución. |
| Ultralytics (YOLO) es AGPL-3.0 | Aplica al código y pesos de Ultralytics, no al dataset. | Se revisa en la HU 05.2 al elegir el modelo. |

## 4. Descarga y preparación

1. En Roboflow Universe, abrir cada dataset → **Download Dataset** → formato **YOLOv8** → *download zip to computer*.
2. Descomprimir en:

   ```
   inference-python/datasets/fuentes/pothole/   (debe contener data.yaml, train/, valid/, test/)
   inference-python/datasets/fuentes/garbage/
   ```

3. Desde `inference-python/`, con el entorno virtual activo:

   ```powershell
   python -m scripts.preparar_dataset --split-original --quitar-duplicados --fuente pothole=datasets/fuentes/pothole --fuente garbage=datasets/fuentes/garbage --limpiar
   python -m scripts.validar_dataset
   ```

El script `preparar_dataset`:

- Con `--split-original` **respeta la división del export** (`train/`, `valid/`, `test/`). Es el modo que usa URBIA: Roboflow aplica sus aumentaciones (ruido, rotación) solo a `train/`, así que `val` y `test` quedan con fotos reales. Sin esa opción vuelve a dividir todo al azar con la semilla 42.
- Con `--quitar-duplicados` **saca de `val` y `test` las fotos que son copia de una foto de un split anterior** (ver sección 5) y registra cuáles en `inference-python/config/duplicados_quitados.csv`, que se versiona en git. `train` nunca se modifica.
- Traduce las clases de cada fuente a las de URBIA (`MAPEO_FUENTES` en `clases.py`) y descarta las demás.
- Convierte polígonos (si el export trae segmentación) a cajas.
- Agrega el prefijo de la fuente a cada archivo (`pothole__…`, `garbage__…`) para que no choquen nombres.
- Genera `datasets/urbia/data.yaml` para Ultralytics y `config/manifiesto_split.csv`.

## 5. Conjuntos de entrenamiento, validación y prueba

| Split | Origen | Uso |
|---|---|---|
| `train` | `train/` del export (incluye las aumentaciones de Roboflow) | Entrenar el modelo (HU 05.2) |
| `val` | `valid/` del export (fotos sin aumentar) | Elegir el mejor *checkpoint* durante el entrenamiento |
| `test` | `test/` del export (fotos sin aumentar) | **Conjunto de evaluación**: solo se usa al final para reportar métricas |

- **Por qué se respeta la división del export:** en la primera versión se dividió todo al azar (70/20/10). Eso mantenía juntas las variantes de cada foto, pero llevaba fotos aumentadas con ruido y rotación a `val` y `test`, y las métricas dejaban de representar fotos reales. Se detectó al inspeccionar errores en la HU 05.2 (ver `modelo.md`, sección 5.1).
- **Sin fuga de datos:** Roboflow divide las fotos antes de aumentarlas, así que ninguna *variante aumentada* de una foto de `train` está en `val` o `test`. Pero el dataset de basura se armó con fotos de internet y **el mismo archivo o la misma foto de banco de imágenes aparece más de una vez** con nombres distintos (por ejemplo, una foto de Alamy con el nombre en francés y en italiano), y Roboflow repartió esas copias en splits distintos. Por eso:
  - `validar_dataset` compara **el contenido de las imágenes, no los nombres** (muchas fotos se llamaban igual en origen, como `image.jpg` o `images46.jpg`, aunque sean distintas). Mismo archivo (SHA-256 idéntico) en dos splits → **error**. Huella visual casi igual (dHash, ≤ 5 de 64 bits) → **advertencia**.
  - `preparar_dataset --quitar-duplicados` saca esas copias de `val` y `test` (se queda la de `train`; si la copia está en `val` y `test`, se queda la de `val`).
- **Identificable:** `inference-python/config/manifiesto_split.csv` (versionado en git) lista cada imagen, su split, su fuente y su SHA-256. `validar_dataset` falla si una imagen de `test` cambia o si aparece una imagen no registrada.

### 5.1 Conjunto de prueba local (Celaya)

Los datasets públicos son de otros países. Para saber si el modelo sirve en Celaya se arma un
conjunto adicional **solo de prueba** (nunca de entrenamiento):

- 30–50 fotos propias de baches y de acumulaciones de basura en Celaya, más ~10 fotos sin incidencias (negativas).
- Tomadas por el equipo con celular, sin personas identificables ni placas legibles.
- Etiquetadas con la guía de la sección 6 y guardadas en `datasets/celaya_test/` con la misma estructura `images/test` y `labels/test`.

## 6. Formato de anotaciones y proceso de etiquetado

**Formato:** YOLO de detección. Un `.txt` por imagen, con el mismo nombre. Una línea por objeto:

```
<id_clase> <x_centro> <y_centro> <ancho> <alto>
0 0.512300 0.633100 0.210000 0.145000
```

Coordenadas normalizadas de 0 a 1. Un `.txt` vacío = imagen sin incidencias (ejemplo negativo, válido).

**Herramienta:** Roboflow Annotate o Label Studio, exportando en formato YOLO.

**Guía de etiquetado**

1. Una caja por bache. Si dos baches se tocan, una caja para cada uno solo si se distinguen los bordes; si no, una caja para ambos.
2. Para `basura`, **una sola caja que cubra todo el montón**, no una por bolsa u objeto.
3. La caja se ajusta al borde visible del objeto, sin dejar mucho espacio vacío alrededor.
4. Objetos cortados por el borde de la foto se etiquetan si se ve al menos la mitad.
5. En caso de duda, no se etiqueta y la foto se anota en una lista de "casos dudosos" para revisarla en equipo.
6. Antes de unir fotos nuevas al dataset, correr `python -m scripts.validar_dataset`.

## 7. Limitaciones conocidas

- **Basura acumulada ≠ objetos individuales.** La clase `basura` representa un *montón* o tiradero, porque eso es lo que un ciudadano reporta. Datasets como TACO etiquetan cada objeto por separado (una botella, una lata). Mezclarlos haría que el modelo marque como incidencia una sola lata tirada, y que divida un montón en decenas de cajas pequeñas. Por eso TACO no se usa, y por eso la guía pide una caja por montón.
- **Cambio de dominio.** Las fotos públicas son de otros países (calles, iluminación, tipo de pavimento distintos a Celaya). Las métricas sobre `test` público pueden ser más altas que en uso real; el conjunto local de la sección 5.1 sirve para medir esa diferencia.
- **Baches con agua o en terracería.** Un bache lleno de agua puede parecer charco, y en calles sin pavimento el "bache" no tiene borde claro. Se espera menor precisión en ambos casos.
- **Desbalance.** Si una clase tiene muchas más cajas que la otra, el modelo tiende a favorecerla. Revisar los conteos de `validar_dataset` antes de entrenar.
- **El modelo solo sugiere.** La detección guarda una sugerencia (`deteccion`); la categoría confirmada del reporte la decide el administrador.

## 8. Atribución

- *Pothole Detection Dataset*, IMACS Pothole Detection, Roboflow Universe, CC BY 4.0.
- *garbage_detection Dataset*, Garbage detection, Roboflow Universe, CC BY 4.0.
- Arya, D. et al. (2022). *RDD2022: A multi-national image dataset for automatic Road Damage Detection*. arXiv:2209.08538. CC BY-SA 4.0 (si se usa).
- Proença, P. F. y Simões, P. (2020). *TACO: Trash Annotations in Context for Litter Detection*. arXiv:2003.06975 (revisado, no usado).

## 9. Resultado de la preparación

### 9.1 Versión actual (`--split-original --quitar-duplicados`)

```
python -m scripts.preparar_dataset --split-original --quitar-duplicados --fuente pothole=... --fuente garbage=... --limpiar
Duplicados quitados de val/test: 24 (4 idénticos, 20 casi idénticos)

python -m scripts.validar_dataset
split   imágenes  negativas    bache   basura
train       6668        228     5513     3933
val          346         11      128      260
test         344         24      176      263

OK: el dataset es válido.
```

| Split | Imágenes | Cajas `bache` | Cajas `basura` | Negativas |
|---|---|---|---|---|
| train | 6 668 | 5 513 | 3 933 | 228 |
| val | 346 | 128 | 260 | 11 |
| test | 344 | 176 | 263 | 24 |
| **Total** | **7 358** | **5 817** | **4 456** | **263** |

- `val` y `test` contienen solo fotos sin aumentar. `train` incluye las 3 variantes aumentadas por foto que genera Roboflow, por eso concentra el 90 % de las imágenes.
- Se quitaron **24 imágenes de `val`/`test`** que eran copia de una foto de un split anterior: 4 archivos idénticos y 20 casi idénticos (la misma foto de banco de imágenes con otro nombre, o cuadros casi iguales de un mismo video). Se revisaron a mano y todas son la misma foto. La lista está en `inference-python/config/duplicados_quitados.csv`.
- El conjunto de evaluación son las **344 imágenes de `test`** registradas en `inference-python/config/manifiesto_split.csv`.

### 9.2 Primera versión (división aleatoria, reemplazada)

```
split   imágenes  negativas    bache   basura
train       5176        176     4065     3144
val         1474         63     1174      910
test         732         24      582      423

OK: el dataset es válido.
```

Formato válido, pero con imágenes aumentadas en `val` y `test`, y sin revisar fotos duplicadas por contenido. Las 7 382 imágenes corresponden a unas 2 780 fotos distintas (1 196 de baches y 1 584 de basura), casi todas con 3 variantes. Se reemplazó por la versión 9.1.