#!/bin/bash

cd ../

GATE_WAY_IMG_NAME="gateway-app:v1"
USERS_SERVICE_API_IMG_NAME="users-app:v1"
MQTT_INIT_CONTAINER="mqtt-init-container-app:v1"
TAIL_WIND_VRSION=4.2.2


echo "📦 Building MQTT Init Container: $MQTT_INIT_CONTAINER..."
# Point to the specific Dockerfile inside the initContainers folder
docker build -t $MQTT_INIT_CONTAINER -f initContainers/mqttInitContainer/Dockerfile .

echo "🔍 Verifying binary existence..."
if ! docker run --rm $MQTT_INIT_CONTAINER ls /usr/local/bin/init-exe >/dev/null 2>&1; then
    echo "❌ ERROR: init-exe NOT FOUND in image! Build failed logic."
    exit 1
fi
echo "✅ Binary confirmed. Proceeding with deployment..."

echo "📦 Building GateWay: $GATE_WAY_IMG_NAME..."
# Point to the app-gateway Dockerfile
docker build -t $GATE_WAY_IMG_NAME -f app-gateway/Dockerfile .

echo "📦 Building Users Service: $USERS_SERVICE_API_IMG_NAME..."
# Point to the api/users Dockerfile
docker build --build-arg TAIL_WIND_VRSION=$TAIL_WIND_VRSION -t $USERS_SERVICE_API_IMG_NAME -f api/users/Dockerfile .
