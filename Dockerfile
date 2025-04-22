# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

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

# Install nginx
RUN apk add --no-cache nginx

# Create run directory for nginx
RUN mkdir -p /run/nginx

WORKDIR /root/

# Copy the Go binary from builder stage
COPY --from=builder /app/enigma /root/enigma

# Copy nginx configuration
COPY config/nginx.conf /etc/nginx/nginx.conf
COPY test/*.pem /usr/share/nginx/cert/

# Copy static assets
COPY public /usr/share/nginx/html/public

# Expose HTTP port
EXPOSE 443


# Launch nginx and the Go application
CMD ["sh", "-c", "nginx && /root/enigma"]
