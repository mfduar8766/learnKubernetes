#!/bin/bash

set -e

echo "Running protoc commnd..."

mkdir -p generated

protoc -I=. \
       -I/usr/local/include \
       --go_out=./generated --go_opt=paths=source_relative \
       --go-grpc_out=./generated --go-grpc_opt=paths=source_relative \
       message.proto
