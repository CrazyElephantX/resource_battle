FROM golang:1.24.5-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl && adduser -D -g '' appuser
USER appuser

WORKDIR /app
COPY --from=build /out/server ./server
COPY --from=build /src/data ./data

EXPOSE 8080
ENTRYPOINT ["./server"]

