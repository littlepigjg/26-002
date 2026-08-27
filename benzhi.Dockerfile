FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./

RUN go mod download || true

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o shurl-server ./cmd/shurl/

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/shurl-server .

RUN adduser -D -u 1000 appuser

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

ENV PORT=8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./shurl-server"]
