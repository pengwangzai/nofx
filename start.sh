#!/bin/bash

# Set environment variables if not already set
export PORT=${PORT:-8080}
export SERVER_HOST=${SERVER_HOST:-0.0.0.0}

# Check if config.json exists, if not copy from example
if [ ! -f config.json ]; then
    echo "Creating config.json from example..."
    cp config.json.example config.json
fi

# Check if .env exists, if not copy from example
if [ ! -f .env ]; then
    echo "Creating .env from example..."
    cp .env.example .env
    echo "Please edit .env file with your configuration"
fi

# Start the application
echo "Starting NOFX server on $SERVER_HOST:$PORT..."
./nofx