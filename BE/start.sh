#!/bin/bash

if [[ "$(docker-compose ps --services | grep -w go-server)" == "go-server" ]]; then
    echo "Service go-server is running"
    docker-compose stop go-server
fi

# Kill any process using port 8080
echo "Killing processes using port 8080..."
lsof -ti :8080 | xargs -r kill -9
lsof -ti :8081 | xargs -r kill -9

echo "Running docker-compose up -d"
# Start Docker Compose
docker-compose up -d

