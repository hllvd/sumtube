#!/bin/sh
set -e

echo "GO_SERVER_HOST=$GO_SERVER_HOST"
echo "GO_SERVER_PORT=$GO_SERVER_PORT"
echo "RENDERER_SERVER_HOST=$RENDERER_SERVER_HOST"
echo "RENDERER_SERVER_PORT=$RENDERER_SERVER_PORT"
echo "TRANSCRIPT_PY_SERVER_HOST=$TRANSCRIPT_PY_SERVER_HOST"
echo "TRANSCRIPT_PY_SERVER_PORT=$TRANSCRIPT_PY_SERVER_PORT"
echo "METADATA_SERVER_HOST=$METADATA_SERVER_HOST"
echo "METADATA_SERVER_PORT=$METADATA_SERVER_PORT"
echo "ENV=$ENV"
echo "DOMAIN=$DOMAIN"
echo "API_SUBDOMAIN=$API_SUBDOMAIN"
echo "ENABLE_HTTPS=$ENABLE_HTTPS"

# Ensure required variables are not empty
for var in GO_SERVER_HOST GO_SERVER_PORT RENDERER_SERVER_HOST RENDERER_SERVER_PORT TRANSCRIPT_PY_SERVER_HOST TRANSCRIPT_PY_SERVER_PORT METADATA_SERVER_HOST METADATA_SERVER_PORT; do
  if [ -z "${!var}" ]; then
    echo "Missing $var environment variable"
    exit 1
  fi
done

# Server name block
if [ "$ENV" = "production" ]; then
  export SERVER_NAME_BLOCK="server_name $API_SUBDOMAIN;"
else
  export SERVER_NAME_BLOCK=""
fi

# Generate base configs from templates
envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/api.conf.template > /etc/nginx/conf.d/api.conf
envsubst '${RENDERER_SERVER_HOST} ${RENDERER_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer.conf.template > /etc/nginx/conf.d/renderer.conf
envsubst '${TRANSCRIPT_PY_SERVER_HOST} ${TRANSCRIPT_PY_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/transcript-py.conf.template > /etc/nginx/conf.d/transcript-py.conf
envsubst '${METADATA_SERVER_HOST} ${METADATA_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/youtube-metadata.conf.template > /etc/nginx/conf.d/youtube-metadata.conf

# Check if certs exist
CERT_PATH="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"
if [ "$ENABLE_HTTPS" = "true" ] && [ -f "$CERT_PATH" ]; then
  echo "Certificates found, enabling HTTPS..."
  cat > /etc/nginx/conf.d/ssl.conf <<EOF
server {
    listen 443 ssl;
    server_name $DOMAIN $API_SUBDOMAIN;

    ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;

    location / {
        proxy_pass http://${GO_SERVER_HOST}:${GO_SERVER_PORT};
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}

server {
    listen 80;
    server_name $DOMAIN $API_SUBDOMAIN;

    # Serve ACME challenge
    location /.well-known/acme-challenge/ {
        root /app/static;
    }

    # Redirect all other HTTP requests to HTTPS
    location / {
        return 301 https://\$host\$request_uri;
    }
}
EOF

elif [ "$ENABLE_HTTPS" = "true" ]; then
  echo "Certificates not found yet, running HTTP-only for ACME challenge..."
  cat > /etc/nginx/conf.d/ssl.conf <<EOF
server {
    listen 80;
    server_name $DOMAIN $API_SUBDOMAIN;

    # Serve ACME challenge
    location /.well-known/acme-challenge/ {
        root /app/static;
    }

    # Serve a simple page for root
    location / {
        return 200 'Certbot will generate HTTPS certificates shortly.';
    }
}
EOF
else
  echo "Running in DEV mode (HTTP only)"
fi

exec nginx -g "daemon off;"
