#!/bin/sh
set -e

# Debug: Print the env vars
echo "GO_SERVER_HOST=$GO_SERVER_HOST"
echo "GO_SERVER_PORT=$GO_SERVER_PORT"

# Ensure variables are not empty
if [ -z "$GO_SERVER_HOST" ] || [ -z "$GO_SERVER_PORT" ]; then
  echo "Missing GO_SERVER_* environment variables"
  exit 1
fi

if [ -z "$RENDERER_SERVER_HOST" ] || [ -z "$RENDERER_SERVER_PORT" ]; then
  echo "Missing RENDERER_SERVER_* environment variables"
  exit 1
fi

# Substitute explicitly
envsubst '${GO_SERVER_HOST} ${GO_SERVER_PORT}' < /etc/nginx/conf.d/api.conf.template > /etc/nginx/conf.d/api.conf
envsubst '${RENDERER_SERVER_HOST} ${RENDERER_SERVER_PORT}' < /etc/nginx/conf.d/renderer.conf.template > /etc/nginx/conf.d/renderer.conf

exec nginx -g "daemon off;"
