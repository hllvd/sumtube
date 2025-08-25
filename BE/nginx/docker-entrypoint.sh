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

# Ensure required variables are not empty
if [ -z "$GO_SERVER_HOST" ] || [ -z "$GO_SERVER_PORT" ]; then
  echo "Missing GO_SERVER_* environment variables"
  exit 1
fi

if [ -z "$RENDERER_SERVER_HOST" ] || [ -z "$RENDERER_SERVER_PORT" ]; then
  echo "Missing RENDERER_SERVER_* environment variables"
  exit 1
fi

if [ -z "$TRANSCRIPT_PY_SERVER_HOST" ] || [ -z "$TRANSCRIPT_PY_SERVER_PORT" ]; then
  echo "Missing TRANSCRIPT_PY_SERVER_* environment variables"
  exit 1
fi

if [ -z "$METADATA_SERVER_HOST" ] || [ -z "$METADATA_SERVER_PORT" ]; then
  echo "Missing METADATA_SERVER_* environment variables"
  exit 1
fi

# Use server_name only in production
if [ "$ENV" = "production" ]; then
  export SERVER_NAME_BLOCK="server_name $API_SUBDOMAIN;"
else
  export SERVER_NAME_BLOCK=""
fi

# Generate final configs

if [ "$ENABLE_API" = "true" ]; then
  echo "ENABLE_API is on"
  if [ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
    echo "Loading ssl config with redirections for $DOMAIN"
    envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/ssl.conf.template > /etc/nginx/conf.d/ssl.conf
  else 
    echo "Loading prod config - without redirection - for $DOMAIN"
    envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/api-prod.conf.template > /etc/nginx/conf.d/api-prod.conf
  fi
else
  echo "Loading devconfig"
   envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/api-dev.conf.template > /etc/nginx/conf.d/api-dev.conf
fi

envsubst '${RENDERER_SERVER_HOST} ${RENDERER_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer.conf.template > /etc/nginx/conf.d/renderer.conf
envsubst '${TRANSCRIPT_PY_SERVER_HOST} ${TRANSCRIPT_PY_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/transcript-py.conf.template > /etc/nginx/conf.d/transcript-py.conf
envsubst '${METADATA_SERVER_HOST} ${METADATA_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/youtube-metadata.conf.template > /etc/nginx/conf.d/youtube-metadata.conf


# check if ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem; exists

if [ "$ENABLE_HTTPS" = "true" ]; then
  echo "Enabling HTTPS for $DOMAIN"
  if [ ! -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
    echo "SSL certificate not found for $DOMAIN"
    
  else
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

    location /.well-known/acme-challenge/ {
        alias /app/static/;
    }

  }
EOF
  fi
  

else
  echo "ENABLE_HTTPS is off"
fi


exec nginx -g "daemon off;"
