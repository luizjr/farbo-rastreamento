# ---- build ----
FROM golang:1.27-alpine AS build

WORKDIR /src

# As dependências mudam menos que o código: cache separado deixa o rebuild rápido.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Binário estático: a imagem final não tem libc.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/tksim ./cmd/tksim

# ---- runtime ----
FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 10001 tracker

COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/tksim /usr/local/bin/tksim

USER tracker

# 8080: API HTTP e WebSocket. 5000: rastreadores.
EXPOSE 8080 5000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/server"]
