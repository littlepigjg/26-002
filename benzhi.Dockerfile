FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./

RUN go mod download

COPY . .

ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -o ubaas-server ./cmd/server/

FROM golang:1.26-alpine

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /app/ubaas-server /usr/local/bin/ubaas-server

COPY . .

RUN adduser -D -u 1000 appuser

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=8080
ENV SERVER_READ_TIMEOUT=30
ENV SERVER_WRITE_TIMEOUT=30
ENV SERVER_IDLE_TIMEOUT=120
ENV SERVER_SHUTDOWN_TIMEOUT=30

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/ubaas-server"]

LABEL org.opencontainers.image.title="UBAAS Server"
LABEL org.opencontainers.image.description="User Behavior Analysis as a Service"
LABEL org.opencontainers.image.version="1.0.0"
LABEL org.opencontainers.image.source="https://github.com/ubaas/ubaas"
