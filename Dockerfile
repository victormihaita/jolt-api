# syntax=docker/dockerfile:1

FROM golang:1.24-alpine

# Create and change to the app directory
WORKDIR /usr/src/app

# Handle dependencies
COPY go.mod ./
COPY go.sum ./

RUN go mod download && go mod verify

# Copy source code
COPY /cmd ./cmd
COPY /internal ./internal
COPY /pkg ./pkg

# Build a static application binary "zalt-api"
RUN go build -v -o /usr/local/bin/zalt-api ./cmd/api

# Expose port
EXPOSE 8080

# Execute zalt-api when the container is started
CMD [ "zalt-api" ]
