#!/bin/bash

set -e

echo "Installing Mockery..."
go install github.com/vektra/mockery/v3@v3.7.3

echo "Executing get_protos.sh to install protoc and Go plugins..."
chmod +x ./get_protos.sh
./get_protos.sh

chmod +x ./updateGoWork.sh
./updateGoWork.sh

cd ..

echo "Run mockery command..."
mockery
