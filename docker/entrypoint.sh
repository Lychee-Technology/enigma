#!/bin/sh
set -e

HTML=/usr/share/nginx/html/public/index.html
CERT_DIR=/usr/share/nginx/cert
CERT_FILE="${CERT_DIR}/_wildcard.enigma.local.pem"
KEY_FILE="${CERT_DIR}/_wildcard.enigma.local-key.pem"

# Inject Cloudflare Turnstile sitekey (set via ENIGMA_TURNSTILE_SITEKEY env var)
if [ -n "$ENIGMA_TURNSTILE_SITEKEY" ]; then
    sed -i "s/__TURNSTILE_SITEKEY__/${ENIGMA_TURNSTILE_SITEKEY}/g" "$HTML"
fi

# Inject Google Analytics measurement ID (set via ENIGMA_GA_ID env var)
if [ -n "$ENIGMA_GA_ID" ]; then
    sed -i "s/__GA_ID__/${ENIGMA_GA_ID}/g" "$HTML"
fi

# Generate a self-signed TLS certificate if none is present.
# Mounting real certificates at /usr/share/nginx/cert will skip this step.
if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then
    echo "No TLS certificate found — generating a self-signed certificate..."
    openssl req -x509 -newkey rsa:4096 \
        -keyout "$KEY_FILE" \
        -out "$CERT_FILE" \
        -days 365 -nodes \
        -subj "/CN=enigma.local/O=Enigma/C=US"
    echo "Self-signed certificate generated."
fi

# Start nginx (runs as root initially, drops to 'nginx' worker user per nginx.conf)
nginx

# Run the Go API server as the non-root enigmauser
exec su-exec enigmauser /app/enigma
