#!/bin/bash

# Check if parameter is provided
if [ $# -eq 0 ]; then
    echo "Usage: ./start.sh [up|down]"
    exit 1
fi

# Get the parameter
action=$1

case $action in
    "up")
        if [[ "$(docker-compose ps --services | grep -w go-server)" == "go-server" ]]; then
            echo "Service go-server is running"
            docker-compose stop go-server
        fi

        # Kill any process using port 8080
        echo "Killing processes using port 8080..."
        lsof -ti :8080 | xargs -r kill -9
        lsof -ti :8081 | xargs -r kill -9

        echo "Running docker-compose up -d --build"
        # Start Docker Compose
        docker-compose up -d --build
        ;;
    "log")
        echo "Restarting containers and showing logs..."
        ./start.sh down && ./start.sh up && docker-compose logs --tail=20 -f -t
        ;;    
    "down")
        echo "Stopping and removing containers..."
        docker-compose down
        ;;
        
    *)
        echo "Invalid parameter. Use 'up' or 'down'"
        exit 1
        ;;
esac
