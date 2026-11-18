# syntax=docker/dockerfile:1

# Versiones alineadas con config/modelo.json -> "entorno"
ARG PYTHON_VERSION=3.11

# =========================================================
# Etapa 1: pesos del modelo (GitHub Release + SHA-256)
# =========================================================
FROM python:${PYTHON_VERSION}-slim-bookworm AS pesos

WORKDIR /app

# Solo lo que usa scripts.descargar_modelo (biblioteca estándar)
COPY config/modelo.json config/
COPY urbia_vision/__init__.py urbia_vision/configuracion.py urbia_vision/metadatos.py urbia_vision/
COPY scripts/__init__.py scripts/descargar_modelo.py scripts/

# URL alternativa opcional; vacío = Release modelo-v<version> indicado en modelo.json
ARG URBIA_MODELO_URL=
# Descarga a models/ y la construcción falla si el SHA-256 no coincide
RUN python -m scripts.descargar_modelo

# =========================================================
# Etapa 2: dependencias de Python
# =========================================================
FROM python:${PYTHON_VERSION}-slim-bookworm AS dependencias

# cpu (por defecto, imagen ligera) o cu121 (perfil GPU, ver docs/vision/docker.md)
ARG TORCH_VARIANTE=cpu
ARG TORCH_VERSION=2.5.1
ARG TORCHVISION_VERSION=0.20.1
ARG ULTRALYTICS_VERSION=8.4.213

ENV PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

RUN python -m venv /opt/venv
ENV PATH=/opt/venv/bin:$PATH

# torch primero desde el índice de PyTorch: así no se instala la variante CUDA de PyPI
RUN pip install "torch==${TORCH_VERSION}" "torchvision==${TORCHVISION_VERSION}" \
      --index-url "https://download.pytorch.org/whl/${TORCH_VARIANTE}"

COPY requirements.txt .
RUN pip install -r requirements.txt "ultralytics==${ULTRALYTICS_VERSION}"

# =========================================================
# Etapa 3: ejecución
# =========================================================
FROM python:${PYTHON_VERSION}-slim-bookworm

# OpenCV (dependencia de Ultralytics) necesita libGL y glib aunque no haya pantalla.
# Usuario sin privilegios: UID 10002 dentro del grupo 10001 (el del backend Go),
# así puede leer las fotos (0640) que Go guarda en el volumen compartido.
RUN apt-get update \
 && apt-get install -y --no-install-recommends libgl1 libglib2.0-0 \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --gid 10001 urbia \
 && useradd --uid 10002 --gid urbia --no-create-home --home-dir /nonexistent \
      --shell /usr/sbin/nologin vision \
 && mkdir -p /app/storage

COPY --from=dependencias /opt/venv /opt/venv

WORKDIR /app

# Código y contrato gRPC generado; todo pertenece a root (solo lectura para vision)
COPY config/modelo.json config/
COPY urbia/ urbia/
COPY urbia_vision/ urbia_vision/
COPY scripts/ scripts/
COPY --from=pesos /app/models/ models/

ENV PATH=/opt/venv/bin:$PATH \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    URBIA_GRPC_DIRECCION=0.0.0.0:50051 \
    URBIA_SALUD_DIRECCION=0.0.0.0:8000 \
    URBIA_DEVICE=auto \
    URBIA_LOG_NIVEL=INFO \
    STORAGE_DIR=/app/storage \
    YOLO_AUTOINSTALL=False \
    YOLO_OFFLINE=True \
    YOLO_CONFIG_DIR=/tmp/Ultralytics \
    MPLCONFIGDIR=/tmp/matplotlib

USER 10002:10001

# 50051: gRPC (solo red interna). 8000: GET /health para Go.
EXPOSE 50051 8000

# 200 solo con el modelo cargado; start-period cubre la carga y el calentamiento en CPU
HEALTHCHECK --interval=30s --timeout=5s --start-period=90s --retries=3 \
  CMD python -c "import os,urllib.request; p=os.environ['URBIA_SALUD_DIRECCION'].rpartition(':')[2]; urllib.request.urlopen(f'http://127.0.0.1:{p}/health', timeout=4)" || exit 1

# Forma exec: Python es el PID 1 y recibe SIGTERM para el apagado ordenado
ENTRYPOINT ["python", "-m", "scripts.servidor_grpc"]