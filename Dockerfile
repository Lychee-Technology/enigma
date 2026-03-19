# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY api/go.mod api/go.sum ./

# Download dependencies
RUN go mod download

# Copy application source from api directory
COPY api/. .

# Build the application binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o enigma ./cmd/main.go

# Stage 2: Final image with Nginx and the Go binary
FROM alpine:latest

# Install nginx, su-exec (privilege drop), and openssl (self-signed cert generation)
RUN apk add --no-cache nginx su-exec openssl

# Create run directory for nginx
RUN mkdir -p /run/nginx

# Create a non-root user for the Go application
RUN addgroup -S enigmagroup && adduser -S -G enigmagroup enigmauser

# Create cert directory — entrypoint generates a self-signed cert here if none is present.
# To use real certificates, mount them at runtime:
#   -v /path/to/certs:/usr/share/nginx/cert:ro
RUN mkdir -p /usr/share/nginx/cert

WORKDIR /app

# Copy the Go binary from builder stage
COPY --from=builder /app/enigma /app/enigma
RUN chown enigmauser:enigmagroup /app/enigma

# Copy nginx configuration
COPY config/nginx.conf /etc/nginx/nginx.conf

# Copy static assets
COPY public /usr/share/nginx/html/public

# Copy entrypoint script
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Expose HTTPS port
EXPOSE 443

# Health check via the /health endpoint on the internal port
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:18080/health || exit 1

# Entrypoint injects env-specific config, starts nginx, then runs Go app as non-root
ENTRYPOINT ["/app/entrypoint.sh"]
