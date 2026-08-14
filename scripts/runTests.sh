#!/bin/bash

set -e

cd ./app-gateway/
echo "In app-gateway package, running go test..."
go test ./...
cd ../

cd ./api/users/
echo "In user service package, running go test..."
go test ./...
cd ../
