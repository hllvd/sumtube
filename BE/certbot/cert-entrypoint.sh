#!/bin/sh
echo "Processing Certbot..."
if [ "$ENABLE_HTTPS" = 'true' ]; then
    certbot certonly --webroot -w /app/static -d sumtube.io -d api.sumtube.io --email vandal.hudson@gmail.com --agree-tos --non-interactive;
else
    echo "Skipping certbot (dev mode)";
fi