# syntax=docker/dockerfile:1

# =========================================================
# Etapa 1: compilación
# =========================================================
ARG GO_VERSION=1.27
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Primero solo go.mod/go.sum: esta capa queda en caché mientras no cambien las dependencias
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Versión que reporta la API en logs y /health (main.version)
ARG VERSION=dev

# CGO_ENABLED=0: binario estático, no depende de la libc de la imagen final
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/urbia-api ./cmd/api

# =========================================================
# Etapa 2: ejecución
# =========================================================
FROM alpine:3.22

# ca-certificates: TLS saliente; tzdata: zona horaria de los logs
# Usuario sin privilegios con UID/GID fijos (10001) para que el volumen
# de fotografías tenga el mismo dueño en todos los equipos.
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 10001 urbia \
 && adduser -S -D -H -u 10001 -G urbia -s /sbin/nologin urbia \
 && mkdir -p /app/storage \
 && chown urbia:urbia /app/storage

WORKDIR /app

# El binario pertenece a root: el proceso (urbia) puede ejecutarlo pero no modificarlo
COPY --from=build /out/urbia-api /app/urbia-api

# Valores por defecto seguros. DATABASE_URL y JWT_SECRET NO van aquí:
# se pasan al ejecutar (--env-file o Compose).
ENV APP_ENV=production \
    GIN_MODE=release \
    LOG_LEVEL=info \
    PORT=8080 \
    STORAGE_DIR=/app/storage \
    TZ=America/Mexico_City

USER 10001:10001

EXPOSE 8080

# Readiness: 200 si la API y PostgreSQL responden (ok/degradado), 503 si la BD cae.
# wget viene en busybox de alpine.
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${PORT}/api/v1/health" || exit 1

# Forma exec: el binario es el PID 1 y recibe SIGTERM para apagarse ordenadamente
ENTRYPOINT ["/app/urbia-api"]