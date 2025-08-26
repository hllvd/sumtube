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

if [ "$ENABLE_HTTPS" = "true" ]; then
  echo "ENABLE_HTTPS is on"
  
  # Handle main domain
  if [ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
    echo "Loading ssl config with redirections for $DOMAIN"
    envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer-ssl.conf.template > /etc/nginx/conf.d/renderer-ssl.conf
  else 
    echo "Loading prod config - without redirection - for $DOMAIN"
    envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer-prod.conf.template > /etc/nginx/conf.d/renderer-prod.conf
  fi

  # Handle api domain
  if [ -f "/etc/letsencrypt/live/$API_SUBDOMAIN/fullchain.pem" ]; then
    echo "Loading ssl config with redirections for $API_SUBDOMAIN"
    envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/ssl.conf.template > /etc/nginx/conf.d/ssl.conf
  else 
    echo "Loading prod config - without redirection - for $DOMAIN and $API_SUBDOMAIN"
    envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/api-prod.conf.template > /etc/nginx/conf.d/api-prod.conf
  fi

else
  echo "Loading devconfig"
   envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/api-dev.conf.template > /etc/nginx/conf.d/api-dev.conf
   envsubst '${RENDERER_SERVER_HOST} ${RENDERER_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer.conf.template > /etc/nginx/conf.d/renderer.conf
fi


envsubst '${TRANSCRIPT_PY_SERVER_HOST} ${TRANSCRIPT_PY_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/transcript-py.conf.template > /etc/nginx/conf.d/transcript-py.conf
envsubst '${METADATA_SERVER_HOST} ${METADATA_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/youtube-metadata.conf.template > /etc/nginx/conf.d/youtube-metadata.conf


exec nginx -g "daemon off;"
