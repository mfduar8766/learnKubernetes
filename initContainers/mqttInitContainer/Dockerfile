# --- STAGE 1: Build ---
FROM golang:1.27.0-alpine3.24 AS builder

WORKDIR /src

# 1. Copy manifests for lib and the init container to cache layers
COPY lib/go.mod lib/go.sum ./lib/
COPY initContainers/mqttInitContainer/go.mod initContainers/mqttInitContainer/go.sum ./initContainers/mqttInitContainer/

# 2. Download dependencies
RUN cd initContainers/mqttInitContainer && go mod download && go mod verify

# 3. Copy the source code
COPY lib/ ./lib/
COPY initContainers/mqttInitContainer/ ./initContainers/mqttInitContainer/

# 4. Build the statically linked binary
# We target the specific folder where main.go lives
# RUN cd initContainers/mqttInitContainer && CGO_ENABLED=0 GOOS=linux go build -o /init-exe .
RUN cd initContainers/mqttInitContainer && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /init-exe .

# --- STAGE 2: Final Image ---
# Switching to plain alpine for a smaller, faster init container
FROM alpine:latest

# Install mosquitto tools (required for mosquitto_passwd)
RUN apk update && apk add --no-cache \
    mosquitto \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Copy the binary from the builder stage
COPY --from=builder /init-exe /usr/local/bin/init-exe

# Verification: ensure mosquitto_passwd and our binary are present
RUN which mosquitto_passwd && ls -l /usr/local/bin/init-exe

# Init containers in K8s usually run a task and exit
CMD ["/usr/local/bin/init-exe"]
