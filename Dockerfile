# ---- Build stage ----
FROM golang:1.24.5-alpine AS build

WORKDIR /src

# Кэшируем зависимости — собираем только при изменении go.mod/go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники и собираем бинарь
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl && \
    adduser -D -g '' appuser

USER appuser

WORKDIR /app

COPY --from=build /out/server ./server
COPY --from=build /src/data ./data

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["./server"]

