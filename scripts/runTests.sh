#!/bin/bash

set -e

echo "Running tests for all packages..."
cd ..

cd ./app-gateway/
echo "In app-gateway package, running go test..."
go test ./...
cd ../

cd ./api/users/
echo "In user service package, running go test..."
go test ./...
cd ../
