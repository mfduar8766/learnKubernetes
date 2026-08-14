#!/bin/bash

echo "Update all go modules..."

cd ./lib/
echo "Running go mod tidy in lib package..."
go mod tidy
cd protos/
echo "Running build.sh in protos package to generate Go code from .proto files..."
chmod +x build.sh
./build.sh
cd ../..

cd ./api/users/
echo "Running go mod tidy in user service package..."
go mod tidy
cd ../..

cd ./app-gateway/
echo "Running go mod tidy in app-gateway package..."
go mod tidy
cd ..

cd ./initContainers/mqttInitContainer/
echo "Running go mod tidy in mqttInitContainer package..."
go mod tidy 
cd ../../

cd ./initContainers/mongoInit/
echo "Running go mod tidy in mongoInitContainer package..."
go mod tidy
cd ../../

cd ./broker
echo "Running go mod tidy in broker package..."
go mod tidy
cd ..

GO_WORK_FILE="go.work"

if [[ ! -f "$GO_WORK_FILE" ]]; then
  echo "Creating go.work file..."
  # Initialize the workspace
  go work init

  # Add all modules to the workspace
  go work use ./api/users ./app-gateway ./initContainers/mqttInitContainer ./lib
  echo "Workspace initialized successfully."
fi
