#!/bin/bash

set -e

cd ../

GATE_WAY_VERSION="v$(cat ./app-gateway/version.txt)"
USERS_SERVICE_API_VERSION="v$(cat ./api/users/version.txt)"
MQTT_INIT_CONTAINER_VERSION="v$(cat ./initContainers/mqttInitContainer/version.txt)"
DOCKER_HUB_USER="mfduar8766"

GATE_WAY_IMG_NAME="app-gateway:$GATE_WAY_VERSION"
USERS_SERVICE_API_IMG_NAME="app-users:$USERS_SERVICE_API_VERSION"
MQTT_INIT_CONTAINER="mqtt-init-container-app:$MQTT_INIT_CONTAINER_VERSION"
TAIL_WIND_VRSION=4.2.2

function verifyDockerImageExists() {
    echo "Verifying image exists..."
    if ! docker image inspect $1 >/dev/null 2>&1; then
        echo "❌ ERROR: $1 NOT FOUND..."
        exit 1
    fi
    echo "✅ Image confirmed for: $1"
}

function buildImageTagAndPush() {
    # 1. Strip any existing tag and prepended path from the local name (e.g., "app-gateway:v1" -> "app-gateway")
    local BASE_NAME=$(echo "$1" | cut -d':' -f1)
    local VERSION=$(echo "$1" | cut -d':' -f2)

    # 2. Extract the short 7-character Git commit SHA
    local GIT_SHA=$(git rev-parse --short HEAD)
    echo "Git SHA: $GIT_SHA"

    # 3. Extract the 12-character image ID
    local FULL_SHA=$(docker images --format "{{.ID}}" "$1" | head -n 1)
    echo "FullImageID: $FULL_SHA"

    if [ -z "$FULL_SHA" ]; then
        echo "❌ Error: Image '$1' not found locally."
        return 1
    fi

    local TIMESTAMP=$(date +%Y_%m_%d_time_%H_%M_%S)
    echo "Timestamp: $TIMESTAMP"

    local NEW_TAG="version_$VERSION_commit_${GIT_SHA}_sha256_${FULL_SHA}_timestamp_${TIMESTAMP}"
    
    # 🌟 UPDATED: Constructs the correct target name including your Docker Hub user profile
    local FULL_TARGET="${DOCKER_HUB_USER}/${BASE_NAME}:${NEW_TAG}"

    echo "✅ Tagging image as: $FULL_TARGET"
    docker tag "$1" "$FULL_TARGET"

    # 🌟 ADDED: Pushes the newly tagged image to your Docker Hub repository
    echo "🚀 Pushing image to Docker Hub..."
    docker push "$FULL_TARGET"

    echo "Deleting original local build image tags..."
    docker rmi "$1"
    
    # Optional: clean up the pushed tag from your local machine to save disk space
    docker rmi "$FULL_TARGET"
}

echo "📦 Building MQTT Init Container: $MQTT_INIT_CONTAINER..."
docker build --no-cache -t $MQTT_INIT_CONTAINER -f initContainers/mqttInitContainer/Dockerfile .
verifyDockerImageExists $MQTT_INIT_CONTAINER
# buildImageTagAndPush $MQTT_INIT_CONTAINER

echo "📦 Building GateWay: $GATE_WAY_IMG_NAME..."
docker build --no-cache --build-arg TAIL_WIND_VRSION=$TAIL_WIND_VRSION -t $GATE_WAY_IMG_NAME -f app-gateway/Dockerfile --progress=plain .
verifyDockerImageExists $GATE_WAY_IMG_NAME
# buildImageTagAndPush $GATE_WAY_IMG_NAME

echo "📦 Building Users Service: $USERS_SERVICE_API_IMG_NAME..."
docker build --no-cache --build-arg TAIL_WIND_VRSION=$TAIL_WIND_VRSION -t $USERS_SERVICE_API_IMG_NAME -f api/users/Dockerfile .
verifyDockerImageExists $USERS_SERVICE_API_IMG_NAME
# buildImageTagAndPush $USERS_SERVICE_API_IMG_NAME
