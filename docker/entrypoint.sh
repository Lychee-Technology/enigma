#!/bin/sh
set -e

HTML=/usr/share/nginx/html/public/index.html

# Inject Cloudflare Turnstile sitekey (set via ENIGMA_TURNSTILE_SITEKEY env var)
if [ -n "$ENIGMA_TURNSTILE_SITEKEY" ]; then
    sed -i "s/__TURNSTILE_SITEKEY__/${ENIGMA_TURNSTILE_SITEKEY}/g" "$HTML"
fi

# Inject Google Analytics measurement ID (set via ENIGMA_GA_ID env var)
if [ -n "$ENIGMA_GA_ID" ]; then
    sed -i "s/__GA_ID__/${ENIGMA_GA_ID}/g" "$HTML"
fi

# Start nginx (runs as root initially, drops to 'nginx' worker user per nginx.conf)
nginx

# Run the Go API server as the non-root enigmauser
exec su-exec enigmauser /app/enigma
