#!/bin/bash

set -e

echo "Checking for protoc compiler..."
if ! command -v protoc &> /dev/null; then
    echo "Installing protoc-gen-go and protoc-gen-go-grpc..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    
    echo "Waiting for 3 seconds to ensure Go binaries are in place..."
    sleep 3

    echo "Adding proto binaries to Go bin to PATH..."
    GOPATH_BIN='$(go env GOPATH)/bin'
    if [[ ":$PATH:" != *":$(go env GOPATH)/bin:"* ]]; then
        echo "export PATH=\"\$PATH:$GOPATH_BIN\"" >> ~/.bashrc
    fi

    echo "protoc could not be found. Installing latest version..."
    # Fetch the download URL for the latest Linux x86_64 asset from GitHub API
    URL=$(curl -s https://api.github.com/repos/protocolbuffers/protobuf/releases/latest \
        | jq -r '.assets[] | select(.name | endswith("linux-x86_64.zip")) | .browser_download_url')
    curl -LO $URL

    sudo unzip $(basename $URL) -d /usr/local
    rm $(basename $URL)

    if [[ ":$PATH:" != *":/usr/local/bin:"* ]]; then
        export PATH="$PATH:/usr/local/bin"
        echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.bashrc
        echo "protoc installed and PATH updated. Please restart your terminal or run 'source ~/.bashrc'."
    else
        echo "protoc installed. /usr/local/bin is already in your PATH."
    fi
else
    echo "protoc compiler and Go dependencies are already installed: $(protoc --version)"

    # 1. Ensure /usr/local/bin is in .bashrc
    if [[ ":$PATH:" != *":/usr/local/bin:"* ]]; then
        echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.bashrc
    fi

    # 2. Ensure Go bin is in .bashrc (This is what fixed the "plugin not found" error)
    GOPATH_BIN='$(go env GOPATH)/bin'
    if [[ ":$PATH:" != *":$(go env GOPATH)/bin:"* ]]; then
        echo "export PATH=\"\$PATH:$GOPATH_BIN\"" >> ~/.bashrc
    fi

    echo "Paths updated in .bashrc file. Running 'source ~/.bashrc' to apply changes..."
    source ~/.bashrc
fi
