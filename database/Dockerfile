# syntax=docker/dockerfile:1

# Imagen de migraciones (HU 10.3): goose + database/migrations

ARG GO_VERSION=1.27
FROM golang:${GO_VERSION}-alpine AS build

ARG GOOSE_VERSION=v3.26.0
# Solo el driver de PostgreSQL: binario más pequeño
RUN CGO_ENABLED=0 go install -trimpath -ldflags "-s -w" \
      -tags "no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
      github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

FROM alpine:3.22

RUN adduser -S -D -H -u 10003 goose

COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY migrations/ /migrations/

USER 10003

# GOOSE_DRIVER, GOOSE_DBSTRING y GOOSE_MIGRATION_DIR llegan desde Compose
ENTRYPOINT ["goose"]
CMD ["up"]