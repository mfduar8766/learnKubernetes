#!/bin/bash
set -e

# Find the repository root dynamically regardless of where the script is called from
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo "ReoRoot: $REPO_ROOT"
cd "$REPO_ROOT"

echo "==> Generating Protobufs in lib..."
if [[ -f "lib/protos/build.sh" ]]; then
  (cd lib/protos && chmod +x build.sh && ./build.sh)
fi

echo "==> Generating Mocks..."
echo "  -> Generating mocks in lib..."
(cd lib && go mod download && mockery)

echo "  -> Generating mocks in api/users..."
(cd api/users && go mod download && mockery)

echo "  -> Generating mocks in app-gateway..."
(cd app-gateway && go mod download && mockery)

echo "==> Running go mod tidy across all modules..."
MODULE_DIRS=(
  "lib"
  "api/users"
  "app-gateway"
  "initContainers/mqttInitContainer"
  "initContainers/mongoInit"
  "broker"
)

for dir in "${MODULE_DIRS[@]}"; do
  if [[ -d "$dir" ]]; then
    echo "  -> Running Go mod tidy in $dir..."
    (cd "$dir" && go mod tidy)
  fi
done

echo "==> Syncing Go Workspace..."
if [[ ! -f "go.work" ]]; then
  echo "  -> Initializing go.work..."
  go work init "${MODULE_DIRS[@]}"
else
  echo "  -> Syncing existing go.work..."
  go work use "${MODULE_DIRS[@]}"
fi

echo "==> Done!"
