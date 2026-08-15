#!/bin/bash

cd ../

GATE_WAY_IMG_NAME="gateway-app:v1"
USERS_SERVICE_API_IMG_NAME="users-app:v1"
MQTT_INIT_CONTAINER="mqtt-init-container-app:v1"
TAIL_WIND_VRSION=4.2.2

function verifyDockerImageExists() {
    echo "Verifying image exists..."
    if ! docker image inspect $1 >/dev/null 2>&1; then
        echo "❌ ERROR: $1 NOT FOUND. Build failed logic."
        exit 1
    fi
    echo "✅ Image confirmed for: $1..."
}

function buildImageTag() {
    # Extract the short 7-character Git commit SHA
    GIT_SHA=$(git rev-parse --short HEAD)
    echo "Git SHA: $GIT_SHA"

    # Output tag: gateway-app:v1-a1b2c3d
    OUT_PUT_TAG="$1@commit:${GIT_SHA}"
    echo "OutTag: $OUT_PUT_TAG"

    # 2. Extract the raw 64-character SHA (removing the "sha256:" prefix for clean tag formatting)
    # docker images --format "{{.ID}}" gateway-app:v1
    FULL_SHA=$(docker images --format "{{.ID}}" gateway-app:v1)
    echo "FullImageSHA: $FULL_SHA"

    # 3. Create a secondary tag containing the SHA
    # Result: gateway-app:v1-sha256-e2815624685c...
    SHA_TAG="@sha256:${FULL_SHA:0:12}"
    echo "SHA_TAG: $SHA_TAG"

    TIMESTAMP=$(date +%Y-%m-%dT%H:%M:%S:%3N)
    echo "Timestamp: $TIMESTAMP"

    # docker tag "$OUT_PUT_TAG""$SHA_TAG"
    echo "✅ Built image tag: $OUT_PUT_TAG$SHA_TAG@timestamp:$TIMESTAMP"
}

echo "📦 Building MQTT Init Container: $MQTT_INIT_CONTAINER..."
# Point to the specific Dockerfile inside the initContainers folder
docker build --no-cache -t $MQTT_INIT_CONTAINER -f initContainers/mqttInitContainer/Dockerfile .
verifyDockerImageExists $MQTT_INIT_CONTAINER
buildImageTag $MQTT_INIT_CONTAINER

# echo "📦 Building GateWay: $GATE_WAY_IMG_NAME..."
# # Point to the app-gateway Dockerfile
docker build --no-cache --build-arg TAIL_WIND_VRSION=$TAIL_WIND_VRSION -t $GATE_WAY_IMG_NAME -f app-gateway/Dockerfile .
verifyDockerImageExists $GATE_WAY_IMG_NAME
buildImageTag $GATE_WAY_IMG_NAME

# echo "📦 Building Users Service: $USERS_SERVICE_API_IMG_NAME..."
# # Point to the api/users Dockerfile
docker build --no-cache --build-arg TAIL_WIND_VRSION=$TAIL_WIND_VRSION -t $USERS_SERVICE_API_IMG_NAME -f api/users/Dockerfile .
verifyDockerImageExists $USERS_SERVICE_API_IMG_NAME
buildImageTag $USERS_SERVICE_API_IMG_NAME
