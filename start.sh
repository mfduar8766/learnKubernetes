#!/bin/bash

# Use a variable for the command
K8S_CMD="minikube kubectl --"
IMAGE_NAME="my-ui-app:v1"

# 1. Environment Setup
eval $(minikube docker-env)

# 2. Build Image 
# Tip: I removed the 'if' so it always picks up code changes (uses Docker cache anyway)
echo "📦 Building image $IMAGE_NAME..."
docker build -t $IMAGE_NAME .

cd k8s

# 3. Apply Dependencies
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

$K8S_CMD apply -f ./ui/ui.deployment.yaml

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

# 6. Port forward
echo "🔗 Connection established at http://127.0.0.1:3000"
$K8S_CMD port-forward svc/ui-service 3000:3000

# 6. Open Service
# echo "🌐 Opening browser..."
# minikube service ui-service
