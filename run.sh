#!/bin/bash

# Jolt API Server Runner
# Usage: ./run.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting Jolt API Server...${NC}"

# Check if .env file exists
if [ ! -f .env ]; then
    echo -e "${YELLOW}Warning: .env file not found. Using environment variables.${NC}"
fi

# Build the application
echo -e "${GREEN}Building...${NC}"
go build -o ./bin/jolt-api ./cmd/api

# Run the server
echo -e "${GREEN}Running server...${NC}"
./bin/jolt-api
