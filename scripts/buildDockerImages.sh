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
    echo "✅ Image confirmed for $1. Proceeding with binary verification..."
}

echo "📦 Building MQTT Init Container: $MQTT_INIT_CONTAINER..."
# Point to the specific Dockerfile inside the initContainers folder
docker build -t $MQTT_INIT_CONTAINER -f initContainers/mqttInitContainer/Dockerfile .
verifyDockerImageExists $MQTT_INIT_CONTAINER

echo "📦 Building GateWay: $GATE_WAY_IMG_NAME..."
# Point to the app-gateway Dockerfile
docker build --no-cache --build-arg TAIL_WIND_VRSION=$TAIL_WIND_VRSION -t $GATE_WAY_IMG_NAME -f app-gateway/Dockerfile .
verifyDockerImageExists $GATE_WAY_IMG_NAME

echo "📦 Building Users Service: $USERS_SERVICE_API_IMG_NAME..."
# Point to the api/users Dockerfile
docker build --no-cache --build-arg TAIL_WIND_VRSION=$TAIL_WIND_VRSION -t $USERS_SERVICE_API_IMG_NAME -f api/users/Dockerfile .
verifyDockerImageExists $USERS_SERVICE_API_IMG_NAME
