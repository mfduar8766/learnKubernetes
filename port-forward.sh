#!/bin/bash

echo "🔍 Fetching pod names..."

# Get Pod Names
K8S_CMD="minikube kubectl --"
MONGO_POD=$($K8S_CMD get pods -l app=mongo -o jsonpath='{.items[0].metadata.name}')
BROKER_POD=$($K8S_CMD get pods -l app=mqtt -o jsonpath='{.items[0].metadata.name}')
REDIS_POD=$($K8S_CMD get pods -l app=redis -o jsonpath='{.items[0].metadata.name}')
# INGRESS_POD=$($K8S_CMD get pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx -o jsonpath='{.items[0].metadata.name}')
# INGRESS_POD=$($K8S_CMD get ingress gateway-ingress -o jsonpath='{.items[0].metadata.name}')

# Check if pods were found to avoid errors
if [ -z "$MONGO_POD" ] || [ -z "$BROKER_POD" ] || [ -z "$REDIS_POD" ]; then
    echo "❌ Error: One or more pods not found. Check your labels!"
    exit 1
fi

echo "🚀 Starting port-forwards..."
$K8S_CMD port-forward "$BROKER_POD" 1883:1883 --address 0.0.0.0 &
$K8S_CMD port-forward "$MONGO_POD" 27017:27017 &
$K8S_CMD port-forward "$REDIS_POD" 6379:6379 &
# $K8S_CMD port-forward "$INGRESS_POD" 80:80 -n ingress-nginx --address 0.0.0.0 &
# $K8S_CMD port-forward -n ingress-nginx service/ingress-nginx-controller 80:80 --address 0.0.0.0

echo "🔐 Starting Ingress Bridge on Port 80..."
echo "$1" | sudo -S -E $K8S_CMD port-forward -n ingress-nginx service/ingress-nginx-controller 80:80 --address 0.0.0.0

echo "✅ Port-forwarding active. Press [CTRL+C] to stop all."

# Wait for background processes so the script stays alive
wait
