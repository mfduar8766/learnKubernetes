#!/bin/bash

GO_WORK_FILE="go.work"

if [[ ! -f "$GO_WORK_FILE" ]]; then
  echo "Creating go.work file..."
  # Initialize the workspace
  go work init

  # Add all modules to the workspace
  go work use ./api/users ./app-gateway ./initContainers/mqttInitContainer ./lib
  echo "Workspace initialized successfully."
fi

read -sp "Enter your sudo password: " MY_PASS
echo "" # Just to move to a new line after typing

MINI_KUBE_WAIT_START_TIME=5

if command -v minikube >/dev/null 2>&1; then
    echo "Minikube is installed: $(minikube version --short)"
else
    echo "Minikube is not installed. Go install it to run the app. https://minikube.sigs.k8s.io/docs/start/?arch=%2Flinux%2Fx86-64%2Fstable%2Fbinary+download"
    exit 1
fi

if ! minikube status | grep "host" | grep -q "Running"; then
  echo "Minikube not running, starting minikube with network policy enabled..."
  minikube start --cni calico
fi

echo "Waiting $MINI_KUBE_WAIT_START_TIME seconds for Minikube API server to be ready..."
until minikube status | grep -q "apiserver: Running"; do
  echo "Still waiting..."
  sleep $MINI_KUBE_WAIT_START_TIME
done

echo "Minikube is up and running!"

if ! minikube addons list | grep "ingress" | grep -v "ingress-dns" | grep -q "enabled"; then
  echo "Installing ingress add on..."
  minikube addons enable ingress
fi

K8S_CMD="minikube kubectl --"
GATE_WAY_IMG_NAME="gateway-app:v1"
USERS_SERVICE_API_IMG_NAME="users-app:v1"
MQTT_INIT_CONTAINER="mqtt-init-container-app:v1"
MONGO_INIT_CONTAINER="mongo-init-container-app:v1"
MY_APP_TEST="myk8sapptest.com"

eval $(minikube docker-env)

# ============================================================================================================================
echo "🧹 Forcing deep clean of MQTT Init..."
# Delete the deployment first so the image isn't "in use"
$K8S_CMD delete deployment mqtt-deployment --ignore-not-found

# Force remove the image from Minikube's Docker daemon
docker rmi -f $MQTT_INIT_CONTAINER || true
# Clean dangling layers to be sure
docker system prune -a -f

echo "Sleeping for 10 seconds..."
sleep 10

echo "🧹 Cleaning old images from Minikube..."
docker rmi $GATE_WAY_IMG_NAME $USERS_SERVICE_API_IMG_NAME $MQTT_INIT_CONTAINER --force || true

echo "⏳ Sleeping for 15 seconds..."
sleep 15

# =================================== Build Docker Containers ==================================================================
# IMPORTANT: All builds must run from the ROOT (current directory) so Docker can see the /lib folder.

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
docker build -t $USERS_SERVICE_API_IMG_NAME -f api/users/Dockerfile .
# ===========================================================================================================================

cd k8s

echo "⚙️ Syncing Config and Secrets..."
# Upload your LOCAL mosquitto.conf
$K8S_CMD delete configmap mosquitto-config --ignore-not-found
$K8S_CMD create configmap mosquitto-config --from-file=mosquitto.conf=./mqtt/mosquitto.conf

# --- Apply Configs & Secrets ---
echo "⚙️ Applying Configs and Secrets..."
$K8S_CMD apply -f ./mongo/mongo.config.yaml
$K8S_CMD apply -f ./mongo/mongo.secret.yaml
$K8S_CMD apply -f ./redis/redis.config.yaml
$K8S_CMD apply -f ./redis/redis.secret.yaml
$K8S_CMD apply -f ./mqtt/mqtt.config.yaml
# $K8S_CMD apply -f ./mqtt/mqtt.secret.yaml

$K8S_CMD delete secret mqtt-secret --ignore-not-found
$K8S_CMD create secret generic mqtt-secret \
  --from-literal=MQTT_USER=user \
  --from-literal=MQTT_PASSWORD=password

$K8S_CMD apply -f networkpolicy.yaml
# =========================================== MONGO INIT ============================================
echo "📦 Uploading initDb.json..."
$K8S_CMD delete configmap mongo-init-data --ignore-not-found
$K8S_CMD create configmap mongo-init-data --from-file=initDb.json=./mongo/initDb.json
# =========================================== PVCs ==================================================
echo "⚙️ Applying Pvcs..."
$K8S_CMD apply -f ./redis/redis.pvc.yaml
$K8S_CMD apply -f ./mongo/mongo.pvc.yaml

$K8S_CMD delete deployment mqtt-deployment --ignore-not-found
echo "⏳ Ensuring MQTT Deployment is fully removed..."
$K8S_CMD wait --for=delete pod -l app=mqtt --timeout=30s
$K8S_CMD delete pvc mqtt-pvc --ignore-not-found
echo "⏳ Ensuring PVC is fully removed..."
$K8S_CMD wait --for=delete pvc/mqtt-pvc --timeout=30s
$K8S_CMD apply -f ./mqtt/mqtt.pvc.yaml
# ===================================================================================================
echo "🚀 Deploying Dbs..."
$K8S_CMD apply -f ./mongo/mongo.deployment.yaml
$K8S_CMD apply -f ./redis/redis.deployment.yaml

echo "🚀 Deploying Broker..."
# Kill old broker to ensure volume is released (prevents deadlock)
$K8S_CMD apply -f ./mqtt/mqtt.deployment.yaml

echo "⏳ Waiting for MQTT Broker to be Ready (1/1)..."
$K8S_CMD rollout status deployment mqtt-deployment --timeout=90s

# Wait for Redis && MongoDb
echo "⏳ Waiting for Databases..."
$K8S_CMD wait --for=condition=ready pod -l app=redis --timeout=60s
$K8S_CMD wait --for=condition=ready pod -l app=mongo --timeout=60s

echo "📥 Importing data into Mongo..."
MONGO_POD=$($K8S_CMD get pods -l app=mongo -o jsonpath='{.items[0].metadata.name}')

echo "Populate MongoDb..."
$K8S_CMD exec $MONGO_POD -- /bin/sh -c "mongoimport --host localhost \
  --username \$MONGO_INITDB_ROOT_USERNAME \
  --password \$MONGO_INITDB_ROOT_PASSWORD \
  --authenticationDatabase admin \
  --db test --collection users --type json \
  --file /data/init/initDb.json --jsonArray \
  --upsert --upsertFields firstName"

echo "🚀 Deleting Old App Deployments..."
$K8S_CMD delete deployment gateway-deployment users-deployment --ignore-not-found

echo "🚀 Deploying Services..."
$K8S_CMD apply -f ./api/users/users.deployment.yaml

echo "🚀 Deploying Client..."
$K8S_CMD apply -f ./app-gateway/app-gateway.deployment.yaml
$K8S_CMD apply -f ./app-gateway/app-gateway.ingress.yaml

echo "🔍 Ingress created..."
$K8S_CMD get ingress gateway-ingress

cd ..

echo "♻️ Restarting UI to pick up latest changes..."
$K8S_CMD rollout restart deployment gateway-deployment
echo "⏳ Waiting for UI to be ready..."
$K8S_CMD rollout status deployment gateway-deployment --timeout=60s

echo "♻️ Restarting Users Service to pick up latest changes..."
$K8S_CMD rollout restart deployment users-deployment
echo "⏳ Waiting for Users to be ready..."
$K8S_CMD rollout status deployment users-deployment --timeout=60s

MINI_IP="127.0.0.1"

echo "🔍 Checking host mapping for webApp..."
if ! grep -q "$MY_APP_TEST" /etc/hosts; then
    echo "Adding $MY_APP_TEST to /etc/hosts..."
    echo "$MY_PASS" | sudo -S tee -a /etc/hosts <<< "$MINI_IP $MY_APP_TEST"
else
    echo "Updating $MY_APP_TEST IP in /etc/hosts..."
    echo "$MY_PASS" | sudo -S sed -i "s/.*$MY_APP_TEST/$MINI_IP $MY_APP_TEST/" /etc/hosts
fi

echo "-------------------------------------------------------"
echo "✅ Done! Application should be at http://$MY_APP_TEST"

echo "🧹 Cleaning up existing port-forwards..."
# The [p] trick prevents the script from killing its own pkill command
echo "$MY_PASS" | sudo -S pkill -9 -f "[p]ort-forward" || true
sleep 2

# Get ONLY the name of the MQTT pod
BROKER_POD=$($K8S_CMD get pods -l app=mqtt -o jsonpath='{.items[0].metadata.name}')

if [ -z "$BROKER_POD" ]; then
    echo "❌ ERROR: MQTT Pod not found. Cannot port-forward."
else
    echo "🔐 Starting Broker on Port 1883 for pod: $BROKER_POD"
    echo "$MY_PASS" | sudo -S -E $K8S_CMD port-forward "$BROKER_POD" 1883:1883 --address 0.0.0.0 &
fi

echo "🔐 Starting Ingress Bridge on Port 80..."
echo "$MY_PASS" | sudo -S -E $K8S_CMD port-forward -n ingress-nginx service/ingress-nginx-controller 80:80 --address 0.0.0.0

# kubectl port-forward -n ingress-nginx service/ingress-nginx-controller 8080:80
# http://myk8sapptest.com:8080/
