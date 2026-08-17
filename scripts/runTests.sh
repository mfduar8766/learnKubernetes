#!/bin/bash

set -e

function runTests() {
    echo "Running unit tests and generating coverage profile..."
    
    if ! go test ./... -coverprofile=coverage.out; then
        echo "❌ Unit tests failed!"
        exit 1
    fi

    MY_NUM=$(go tool cover -func=coverage.out | awk '/total:/ {gsub(/%/, "", $NF); print $NF}')

    # Default to 0 if MY_NUM failed to parse or is empty
    MY_NUM=${MY_NUM:-0}

    echo "Total Code Coverage: ${MY_NUM}%"

    # Set your minimum required coverage threshold (e.g., 6%)
    THRESHOLD=0.6

    # Compare floats using bc
    if [ $(echo "$MY_NUM <= $THRESHOLD" | bc) -eq 1 ]; then
        echo "❌ Coverage is less than or equal to ${THRESHOLD}% (Current: ${MY_NUM}%)"
        exit 1
    else
        echo "✅ Coverage check passed!"
    fi

    cd ../
}

echo "Running tests for all packages..."
cd ..

cd ./lib/
echo "In Lib package, running go test..."
runTests

cd ./app-gateway/
echo "In app-gateway package, running go test..."
runTests

cd ./api/users/
echo "In user service package, running go test..."
runTests
