# Multi-stage build for minimal, secure production container
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Download dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# Minimal Alpine runtime
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/server .

# Run as non-root user
RUN adduser -D -u 10001 appuser
USER appuser

EXPOSE 8080
ENV PORT=8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/healthz || exit 1

ENTRYPOINT ["./server"]
