# Prioridad explicable (HU 07.1)

El administrador obtiene una **prioridad provisional** para cada reporte, con la
explicación factor por factor. El sistema **no inventa datos**: lo que no se sabe
queda como "no disponible" y no suma ni resta.

> ⚠️ **Los pesos son provisionales.** La fórmula `v1-provisional` es una propuesta
> inicial para el MVP. Antes de usarla como criterio oficial debe validarse con
> reportes reales y con el área responsable (por ejemplo, comparando la prioridad
> calculada con la que asignaría una persona experta). Por eso cada cálculo guarda
> `version_formula` y la respuesta incluye `"provisional": true`.

## Endpoints

| Método | Ruta | Qué hace |
|---|---|---|
| `POST` | `/api/v1/reportes/{id}/prioridad` | Calcula (o recalcula), guarda el resultado y, si los datos alcanzan, asigna la prioridad al reporte. `201` |
| `GET` | `/api/v1/reportes/{id}/prioridad` | Devuelve el último cálculo y si quedó desactualizado. No guarda nada. `200` |

- Solo `administrador` (`dominio.Rol.PuedeCalcularPrioridad`); un ciudadano recibe `403`.
- `404` si el reporte no existe · `409` si está `resuelto` (solo en `POST`).
- `400 VALIDACION_FALLIDA` si la evaluación está fuera de rango o falta la justificación.

Cuerpo de `POST` (todo opcional):

```json
{
  "gravedad_observable": 4,
  "afectacion_estimada": 3,
  "justificacion_afectacion": "Avenida principal con paso de transporte público"
}
```

Sin cuerpo se **recalcula** con los datos actuales y la evaluación anterior del
administrador. Un campo omitido reutiliza el valor anterior (son datos que el
propio administrador declaró, no valores inventados).

## Fórmula

| Factor | Peso | Fuente | Valor (0 a 1) | Si no hay dato |
|---|---|---|---|---|
| `gravedad_observable` | 0.40 | Evaluación del administrador (1-5) al revisar la evidencia | `(n − 1) / 4` | No disponible |
| `afectacion_estimada` | 0.30 | Evaluación del administrador (1-5) **con justificación** | `(n − 1) / 4` | No disponible |
| `recurrencia_verificada` | 0.20 | PostGIS: reportes de **otros ciudadanos**, misma categoría confirmada, a ≤ `DUPLICADOS_RADIO_M` y ± `DUPLICADOS_VENTANA_DIAS` (HU 06.5) | `min(n, 4) / 4` | No verificable si el reporte no tiene categoría |
| `antiguedad` | 0.10 | `reporte.fecha_creacion` | `min(días, 30) / 30` | Siempre disponible |

```
puntuación = 100 · Σ(peso · valor) / Σ(peso de los factores disponibles)
cobertura  = Σ(peso de los factores disponibles)
```

Los pesos de los factores faltantes se **reparten** entre los disponibles
(`peso_efectivo`), y cada factor informa cuántos puntos aporta (`aporte`). La suma
de los aportes es la puntuación.

### Por qué la IA no decide la gravedad ni la afectación

- El modelo YOLO **sugiere la categoría** (bache, basura…), no mide tamaño, profundidad ni riesgo.
- La **afectación real** (tránsito, peatones, servicios, zona escolar) no se ve en una
  fotografía: depende del contexto. Por eso solo la estima el administrador y debe
  escribir en qué se basa (`justificacion_afectacion`, 10-500 caracteres). La BD lo
  exige con `ck_calculo_prioridad_justificacion`.
- Una recurrencia `0` **sí** es un dato (se verificó y no hay otros reportes);
  "no verificable" es distinto y no cuenta.
- Un mismo ciudadano cuenta una sola vez en la recurrencia.

## Escala de salida y datos faltantes

| Puntuación | Nivel |
|---|---|
| ≥ 67 | `alta` |
| 34 – 66.9 | `media` |
| < 34 | `baja` |

- Si la **cobertura** es menor que `PRIORIDAD_COBERTURA_MIN` (0.50), el resultado
  tiene `"nivel": null` y `"datos_suficientes": false`: se guarda el cálculo para
  auditoría, pero **no** se modifica `reporte.prioridad`.
- Solo con recurrencia y antigüedad la cobertura es 0.30 → insuficiente. Basta con que
  el administrador evalúe la gravedad (0.40 + 0.10 = 0.50) para obtener un nivel.

| Caso | Cobertura | Puntuación | Nivel |
|---|---|---|---|
| Gravedad 5, afectación 3, 2 recurrentes, 15 días | 1.00 | 70.0 | alta |
| Gravedad 4, sin categoría, 3 días | 0.50 | 62.0 | media |
| Gravedad 1, 0 recurrentes, recién creado | 0.70 | 0.0 | baja |
| Sin evaluación, 4 recurrentes, 60 días | 0.30 | — | `null` (insuficiente) |

## Qué se guarda

Migración `00008_calculo_prioridad.sql`:

- **`calculo_prioridad`**: una fila por cálculo con las entradas (evaluaciones,
  categoría, recurrencias, días), el resultado (cobertura, puntuación, nivel, resumen)
  y `factores` (JSONB) con la explicación completa. Nunca se sobrescribe: es historial.
- **`reporte.origen_prioridad`**: `por_defecto` (el `media` con que nace todo reporte,
  que **no** es un cálculo) o `calculada`.

## Recalcular

`GET` compara el último cálculo con lo que daría la fórmula hoy y explica qué cambió
en `vigencia.motivos`:

- cambió la categoría del reporte (HU 06.3),
- cambió la recurrencia verificada (llegaron reportes cercanos, HU 06.5),
- con la antigüedad actual el nivel pasaría a otro,
- cambió la versión de la fórmula.

Si `vigencia.desactualizado` es `true`, el administrador hace `POST` (sin cuerpo) para
recalcular.

## Configuración

```env
PRIORIDAD_PESO_GRAVEDAD=0.40
PRIORIDAD_PESO_AFECTACION=0.30
PRIORIDAD_PESO_RECURRENCIA=0.20
PRIORIDAD_PESO_ANTIGUEDAD=0.10
PRIORIDAD_ANTIGUEDAD_MAX_DIAS=30
PRIORIDAD_RECURRENCIA_MAX=4
PRIORIDAD_COBERTURA_MIN=0.50
```

La API no arranca si los pesos no suman 1 o algún valor está fuera de rango.
El radio y la ventana de la recurrencia son los de la HU 06.5 (`DUPLICADOS_*`).