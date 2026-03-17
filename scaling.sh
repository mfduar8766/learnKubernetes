#!/bin/bash

K8S_CMD="minikube kubectl --"

echo "Scale up/down: (u/d)"

read scaleUpDown

if [ $scaleUpDown = u ]; then
  echo "Scale all pods up..." &
  $K8S_CMD scale deployment ui-deployment mongo-deployment redis-deployment --replicas=1
elif [ $scaleUpDown = d ]; then
  echo "Scale all pods down..." &
  $K8S_CMD scale deployment ui-deployment mongo-deployment redis-deployment --replicas=0
else
    echo "Option $scaleUpDown not supported..."
fi
