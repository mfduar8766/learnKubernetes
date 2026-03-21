#!/bin/bash

# Use a variable for the command
K8S_CMD="minikube kubectl --"

GATE_WAY_IMG_NAME="gateway-app:v1"
USERS_SERVICE_API_IMG_NAME="users-app:v1"

MY_APP_TEST="myk8sapptest.com"
RMQ_DOMAIN="rmq.myk8sapptest.com"

read -sp "Enter your sudo password: " MY_PASS
echo "" # Just to move to a new line after typing

echo "Running templ to build before building docker image..."
go tool templ generate

# 1. Environment Setup
eval $(minikube docker-env)

# 2. Build Image 
# echo "📦 Building Gateway: $GATE_WAY_IMG_NAME..."
# docker build -t $GATE_WAY_IMG_NAME .

# # 3. Build Users Service (STAY AT ROOT, point to the subfolder Dockerfile)
# echo "📦 Building Users Service: $USERS_SERVICE_API_IMG_NAME..."
# docker build -t $USERS_SERVICE_API_IMG_NAME -f api/users/Dockerfile .

echo "🧹 Cleaning old images from Minikube..."
docker rmi $GATE_WAY_IMG_NAME $USERS_SERVICE_API_IMG_NAME --force || true

# Now build
echo "📦 Building GateWay: $GATE_WAY_IMG_NAME..."
docker build -t $GATE_WAY_IMG_NAME .
echo "📦 Building Users Service: $USERS_SERVICE_API_IMG_NAME..."
docker build -t $USERS_SERVICE_API_IMG_NAME -f api/users/Dockerfile .

cd k8s

# 3. Apply Dependencies, order matters config->secret->deployment
echo "⚙️ Applying Configs and Secrets..."
$K8S_CMD apply -f ./mongo/mongo.config.yaml
$K8S_CMD apply -f ./mongo/mongo.secret.yaml
$K8S_CMD apply -f ./redis/redis.config.yaml
$K8S_CMD apply -f ./redis/redis.secret.yaml
$K8S_CMD apply -f ./rmq/rmq.config.yaml
$K8S_CMD apply -f ./rmq/rmq.secret.yaml

echo "📦 Uploading initDb.json to Kubernetes..."
# $K8S_CMD delete configmap mongo-init-data --ignore-not-found
# $K8S_CMD create configmap mongo-init-data --from-file=./mongo/initDb.json
$K8S_CMD delete configmap mongo-init-data --ignore-not-found
# We name the key 'init-script.sh' so Mongo executes it on startup
$K8S_CMD create configmap mongo-init-data --from-file=initDb.json=./mongo/initDb.json

# MUST BE BEFORE DEPLOYMENT
echo "⚙️ Applying Pvcs..."
$K8S_CMD apply -f ./redis/redis.pvc.yaml
$K8S_CMD apply -f ./mongo/mongo.pvc.yaml

# 4. Refresh Deployment
echo "🚀 Deploying Dbs..."
$K8S_CMD apply -f ./mongo/mongo.deployment.yaml
$K8S_CMD apply -f ./redis/redis.deployment.yaml

echo "🚀 Deploying Broker..."
$K8S_CMD apply -f ./rmq/rmq.deployment.yaml

# Wait for Redis && MongoDb
echo "⏳ Waiting for Databases..."
$K8S_CMD wait --for=condition=ready pod -l app=redis --timeout=60s
$K8S_CMD wait --for=condition=ready pod -l app=mongo --timeout=60s

echo "📥 Importing data into Mongo..."
# Get the pod name
MONGO_POD=$($K8S_CMD get pods -l app=mongo -o jsonpath='{.items[0].metadata.name}')

# Run the import directly
# Note the SINGLE QUOTES around the sh -c command. 
# This ensures the variables are evaluated INSIDE the container.
echo "Populate Db..."
$K8S_CMD exec $MONGO_POD -- /bin/sh -c "mongoimport --host localhost \
  --username \$MONGO_INITDB_ROOT_USERNAME \
  --password \$MONGO_INITDB_ROOT_PASSWORD \
  --authenticationDatabase admin \
  --db test --collection users --type json \
  --file /data/init/initDb.json --jsonArray \
  --upsert --upsertFields firstName"

echo "🚀 Deleting Deployments..."
$K8S_CMD delete deployment gateway-deployment users-deployment ui-deployment --ignore-not-found

echo "🚀 Deploying Services..."
$K8S_CMD apply -f ./api/users/users.deployment.yaml

echo "🚀 Deploying Client..."
$K8S_CMD apply -f ./app-gateway/app-gateway.deployment.yaml
$K8S_CMD apply -f ./app-gateway/app-gateway.ingress.yaml

echo "🔍 Ingress created..."
$K8S_CMD get ingress gateway-ingress

cd ..

# ONLY restart the UI to pick up the new image/code
# ROLLOUT CMD IS PREFERED
# Zero Downtime: It starts a new pod before killing the old one.
# Cleaner: It doesn't "break" the connection; it just swaps the underlying containers.
# Updates Envs: It forces the pod to re-read the ConfigMaps and Secrets.
echo "♻️ Restarting UI to pick up latest changes..."
$K8S_CMD rollout restart deployment gateway-deployment

# 5. Use rollout status instead of 'wait pod' for deployments
echo "⏳ Waiting for UI to be ready..."
$K8S_CMD rollout status deployment gateway-deployment --timeout=60s

echo "♻️ Restarting Users Service to pick up latest changes..."
$K8S_CMD rollout restart deployment users-deployment

echo "⏳ Waiting for Users to be ready..."
$K8S_CMD rollout status deployment users-deployment --timeout=60s

# Get the IP
# MINI_IP=$(minikube ip)

# FOrce it to be localhost
MINI_IP="127.0.0.1"

echo "🔍 Checking host mapping for webApp..."
# Use sudo only for the specific host check/update logic
if ! grep -q "$MY_APP_TEST" /etc/hosts; then
    echo "Adding $MY_APP_TEST to /etc/hosts..."
    echo "$MY_PASS" | sudo -S tee -a /etc/hosts <<< "$MINI_IP $MY_APP_TEST"
else
    echo "Updating $MY_APP_TEST IP in /etc/hosts..."
    echo "$MY_PASS" | sudo -S sed -i "s/.*$MY_APP_TEST/$MINI_IP $MY_APP_TEST/" /etc/hosts
fi

echo "🔍 Checking host mapping for RMQ..."
if ! grep -q "$RMQ_DOMAIN" /etc/hosts; then
    echo "Adding $RMQ_DOMAIN to /etc/hosts..."
    echo "$MY_PASS" | sudo -S tee -a /etc/hosts <<< "$MINI_IP $RMQ_DOMAIN"
else
    echo "Updating $RMQ_DOMAIN IP in /etc/hosts..."
    echo "$MY_PASS" | sudo -S sed -i "s/.*$RMQ_DOMAIN/$MINI_IP $RMQ_DOMAIN/" /etc/hosts
fi

echo "-------------------------------------------------------"
echo "✅ Done! Application should be at http://$MY_APP_TEST"
echo "✅ Done! RMQ UI Application should be at http://$RMQ_DOMAIN"

# Kill any existing port-forwards
echo "Killing existing port-forwards..."
echo "$MY_PASS" | sudo -S pkill -f "port-forward -n ingress-nginx"

echo "🔐 Starting Ingress Bridge on Port 80..."
# Use -S to read the password from the pipe
echo "$MY_PASS" | sudo -S -E $K8S_CMD port-forward -n ingress-nginx deployment/ingress-nginx-controller 80:80

# IMPORTANT IF USING TUNNEL (minikube tunnel) YOU NEED TO RUN THAT FIRST THEN THE THIS SCRIPT
# Check if tunnel is running, if not, start it
# if ! pgrep -f "minikube tunnel" > /dev/null; then
#     echo "⚠️  'minikube tunnel' is not running."
#     echo "🔐 Starting tunnel now (this will keep this terminal open)..."
#     minikube tunnel
# else
#     echo "✅ 'minikube tunnel' is already running in another terminal."
#     echo "🌐 You can visit http://$MY_APP_TEST now!"
# fi

# 6. Port forward IF NOT USING INGRESS
# echo "🔗 Connection established at http://127.0.0.1:3000"
# $K8S_CMD port-forward svc/ui-service 3000:3000

# 6. Open Service
# echo "🌐 Opening browser..."
# minikube service ui-service


#kubectl port-forward deployment/mongo-deployment 27017:27017
