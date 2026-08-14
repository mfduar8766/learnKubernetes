#!/bin/bash

set -e

echo "Executing get_protos.sh to install protoc and Go plugins..."
chmod +x ./get_protos.sh
./get_protos.sh

chmod +x ./updateGoWork.sh
./updateGoWork.sh
