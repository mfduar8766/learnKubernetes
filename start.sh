#!/bin/bash

# Use a variable for the command
K8S_CMD="minikube kubectl --"
IMAGE_NAME="my-ui-app:v1"
MY_APP_TEST="myk8sapptest.com"

read -sp "Enter your sudo password: " MY_PASS
echo "" # Just to move to a new line after typing

echo "Running templ to build before building docker image..."

go tool templ generate

# 1. Environment Setup
eval $(minikube docker-env)

# 2. Build Image 
# Tip: I removed the 'if' so it always picks up code changes (uses Docker cache anyway)
echo "📦 Building image $IMAGE_NAME..."
docker build -t $IMAGE_NAME .

cd k8s

# 3. Apply Dependencies, order matters config->secret->deployment
echo "⚙️ Applying Configs and Secrets..."
$K8S_CMD apply -f ./mongo/mongo.config.yaml
$K8S_CMD apply -f ./mongo/mongo.secret.yaml
$K8S_CMD apply -f ./redis/redis.config.yaml
$K8S_CMD apply -f ./redis/redis.secret.yaml

# 4. Refresh Deployment
echo "🚀 Deploying Dbs..."
$K8S_CMD apply -f ./mongo/mongo.deployment.yaml
$K8S_CMD apply -f ./redis/redis.deployment.yaml

# Wait for Redis && MongoDb
echo "⏳ Waiting for Databases..."
$K8S_CMD wait --for=condition=ready pod -l app=redis --timeout=60s
$K8S_CMD wait --for=condition=ready pod -l app=mongo --timeout=60s

echo "🚀 Deploying Client..."
$K8S_CMD apply -f ./ui/ui.deployment.yaml
$K8S_CMD apply -f ./ui/ui.ingress.yaml

echo "🔍 Ingress created..."
$K8S_CMD get ingress ui-ingress

cd ..

# ONLY restart the UI to pick up the new image/code
# ROLLOUT CMD IS PREFERED
# Zero Downtime: It starts a new pod before killing the old one.
# Cleaner: It doesn't "break" the connection; it just swaps the underlying containers.
# Updates Envs: It forces the pod to re-read the ConfigMaps and Secrets.
echo "♻️ Restarting UI to pick up latest changes..."
$K8S_CMD rollout restart deployment ui-deployment

# 5. Use rollout status instead of 'wait pod' for deployments
echo "⏳ Waiting for UI to be ready..."
$K8S_CMD rollout status deployment ui-deployment --timeout=60s

# Get the IP
# MINI_IP=$(minikube ip)

# FOrce it to be localhost
MINI_IP="127.0.0.1"

echo "🔍 Checking host mapping..."
# Use sudo only for the specific host check/update logic
if ! grep -q "$MY_APP_TEST" /etc/hosts; then
    echo "Adding $MY_APP_TEST to /etc/hosts..."
    echo "$MY_PASS" | sudo -S tee -a /etc/hosts <<< "$MINI_IP $MY_APP_TEST"
else
    echo "Updating $MY_APP_TEST IP in /etc/hosts..."
    echo "$MY_PASS" | sudo -S sed -i "s/.*$MY_APP_TEST/$MINI_IP $MY_APP_TEST/" /etc/hosts
fi

echo "-------------------------------------------------------"
echo "✅ Done! Application should be at http://$MY_APP_TEST"

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
