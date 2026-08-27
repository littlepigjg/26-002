# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod ./

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /src/server ./cmd/server

# Runtime stage
FROM golang:1.26-alpine

RUN apk --no-cache add ca-certificates tzdata wget

COPY --from=builder /src/server /usr/local/bin/server

RUN chmod +x /usr/local/bin/server

WORKDIR /app

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["/usr/local/bin/server"]
