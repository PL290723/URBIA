# Imagen Docker del servicio de inferencia (HU 10.2)

## Construcción
Tres etapas sobre `python:3.11-slim-bookworm`:

1. **pesos**: `scripts.descargar_modelo` baja los pesos del Release `modelo-v<version>`
   y verifica el SHA-256 de `config/modelo.json`. Si no coincide, la construcción falla.
2. **dependencias**: virtualenv con torch 2.5.1 / torchvision 0.20.1 (CPU por defecto)
   y `requirements.txt` con ultralytics 8.4.213, las versiones registradas en `modelo.json`.
3. **ejecución**: código, contrato gRPC generado (`urbia/`) y pesos. Sin compiladores ni caché de pip.

```powershell
docker build -t urbia-inferencia:0.1.0 .\inference-python
```

| Argumento | Por defecto | Para qué |
|---|---|---|
| `TORCH_VARIANTE` | `cpu` | `cu121` para el perfil GPU |
| `URBIA_MODELO_URL` | vacío | URL alternativa de los pesos |
| `ULTRALYTICS_VERSION` | `8.4.213` | igual que `modelo.json` |

## Pesos del modelo
- **Incorporados** (por defecto): van dentro de la imagen en `/app/models`. La imagen
  funciona sin internet, lo que conviene en la feria.
- **Montados** (para probar otros pesos sin reconstruir): actualizar `config/modelo.json` y montar
  `-v .\inference-python\models:/app/models:ro`. El SHA-256 se valida igual al arrancar.

## Ejecución
| Aspecto | Valor |
|---|---|
| Usuario | `vision` (UID 10002), grupo `urbia` (GID 10001, el del backend Go) |
| gRPC | `0.0.0.0:50051`. No se publica fuera de la red de Docker |
| Salud | `GET :8000/health`, 200 solo con el modelo cargado |
| Fotografías | volumen `urbia_storage` en `/app/storage` **de solo lectura** |
| Ultralytics | `YOLO_AUTOINSTALL=False`, `YOLO_OFFLINE=True` |
| Apagado | SIGTERM, con `URBIA_APAGADO_GRACIA` (10 s). Usar `docker stop -t 15` |

Del lado de Go: `INFERENCE_URL=http://urbia-inferencia:8000` e
`INFERENCE_GRPC_ADDR=urbia-inferencia:50051`.

## Perfil GPU (opcional)
La imagen CPU es la predeterminada: una foto por reporte tarda del orden de 100 ms con
`yolo26s`. Para usar la GPU NVIDIA (Docker Desktop con WSL 2 y el driver de Windows al día):

```powershell
docker build -t urbia-inferencia:0.1.0-gpu --build-arg TORCH_VARIANTE=cu121 .\inference-python
docker run -d --name urbia-inferencia --network urbia-red --gpus all -e URBIA_DEVICE=0 `
  -v urbia_storage:/app/storage:ro urbia-inferencia:0.1.0-gpu
```

La imagen GPU pesa varios GB más (incluye las bibliotecas de CUDA). En el log de arranque
debe aparecer `dispositivo=0`. `URBIA_DEVICE=auto` también elige la GPU si la ve.