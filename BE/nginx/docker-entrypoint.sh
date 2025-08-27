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
echo "HTTPS_ENABLE=$HTTPS_ENABLE"

# Use server_name only in production
if [ "$ENV" = "production" ]; then
  export SERVER_NAME_BLOCK="server_name $API_SUBDOMAIN;"
else
  export SERVER_NAME_BLOCK=""
fi


generate_nginx_conf() {
  if [ $# -ne 2 ]; then
    echo "Usage: generate_nginx_conf <template-file> <output-file>"
    return 1
  fi

  local template="$1"
  local output="$2"

  if [ ! -f "$template" ]; then
    echo "Error: template file '$template' not found."
    return 1
  fi

  # Define which variables you want to substitute
  envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} \
            ${RENDERER_SERVER_HOST} ${RENDERER_SERVER_PORT} \
            ${TRANSCRIPT_PY_SERVER_HOST} ${TRANSCRIPT_PY_SERVER_PORT} \
            ${METADATA_SERVER_HOST} ${METADATA_SERVER_PORT} \
            ${ENV} ${DOMAIN} ${API_SUBDOMAIN} ${HTTPS_ENABLE} ${SERVER_NAME_BLOCK}' \
    < "$template" > "$output"

  echo "Config generated: $output"
}

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



# Generate final configs

if [ "$HTTPS_ENABLE" = "true" ]; then
  echo "HTTPS_ENABLE is on"
  
  # Handle main domain
  if [ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
    echo "Loading[$DOMAIN]:/etc/nginx/conf.d/renderer-ssl.conf.template"
    generate_nginx_conf /etc/nginx/conf.d/renderer-ssl.conf.template /etc/nginx/conf.d/renderer-ssl.conf
    # envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer-ssl.conf.template > /etc/nginx/conf.d/renderer-ssl.conf
  else 
    echo "Loading[$DOMAIN]:/etc/nginx/conf.d/renderer-prod.conf.template"
    generate_nginx_conf /etc/nginx/conf.d/renderer-prod.conf.template /etc/nginx/conf.d/renderer-prod.conf
    #envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer-prod.conf.template > /etc/nginx/conf.d/renderer-prod.conf
  fi

  # Handle api domain
  if [ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
    echo "Loading[$DOMAIN]:/etc/nginx/conf.d/ssl.conf.template"
    generate_nginx_conf /etc/nginx/conf.d/ssl.conf.template /etc/nginx/conf.d/ssl.conf
    # envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/ssl.conf.template > /etc/nginx/conf.d/ssl.conf
  else 
    echo "Loading[$DOMAIN]:/etc/nginx/conf.d/api-prod.conf.template"
    generate_nginx_conf /etc/nginx/conf.d/api-prod.conf.template /etc/nginx/conf.d/api-prod.conf
    # envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/api-prod.conf.template > /etc/nginx/conf.d/api-prod.conf
  fi

else
  echo "Loading devconfig /etc/nginx/conf.d/api-dev.conf.template and /etc/nginx/conf.d/renderer-dev.conf.template"
  generate_nginx_conf /etc/nginx/conf.d/api-dev.conf.template /etc/nginx/conf.d/api-dev.conf
  #envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/api-dev.conf.template > /etc/nginx/conf.d/api-dev.conf
  generate_nginx_conf /etc/nginx/conf.d/renderer-dev.conf.template /etc/nginx/conf.d/renderer-dev.conf
  # envsubst '${RENDERER_SERVER_HOST} ${GO_SERVER_HOST} ${GO_SERVER_PORT} ${RENDERER_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/renderer-dev.conf.template > /etc/nginx/conf.d/renderer-dev.conf
fi

generate_nginx_conf /etc/nginx/conf.d/transcript-py.conf.template /etc/nginx/conf.d/transcript-py.conf
generate_nginx_conf /etc/nginx/conf.d/youtube-metadata.conf.template /etc/nginx/conf.d/youtube-metadata.conf
#envsubst '${TRANSCRIPT_PY_SERVER_HOST} ${TRANSCRIPT_PY_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/transcript-py.conf.template > /etc/nginx/conf.d/transcript-py.conf
#envsubst '${METADATA_SERVER_HOST} ${METADATA_SERVER_PORT} ${SERVER_NAME_BLOCK}' < /etc/nginx/conf.d/youtube-metadata.conf.template > /etc/nginx/conf.d/youtube-metadata.conf

echo "Starting nginx..."
exec nginx -g "daemon off;"
