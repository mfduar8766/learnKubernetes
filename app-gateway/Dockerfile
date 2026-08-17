# --- STAGE 1: Build ---
FROM golang:1.26.2-alpine AS builder

WORKDIR /src

ARG TAIL_WIND_VRSION=4.2.2

ENV PATH="/go/bin:${PATH}"

RUN echo "Using Tailwind CSS version: $TAIL_WIND_VRSION"

# 1. Install dependencies (added protoc-dev for standard proto definitions)
RUN apk add --no-cache bash protoc protobuf-dev

# Create symlink so /usr/local/include finds standard Google proto files
RUN mkdir -p /usr/local/include && \
    ln -s /usr/include/google /usr/local/include/google

# 2. Install your Go tools
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/a-h/templ/cmd/templ@latest && \
    go install github.com/vektra/mockery/v3@v3.7.3

ADD https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAIL_WIND_VRSION}/tailwindcss-linux-x64 ./tailwindcss
RUN chmod +x ./tailwindcss

# 3. Copy manifests
COPY lib/go.mod lib/go.sum ./lib/
COPY app-gateway/go.mod app-gateway/go.sum ./app-gateway/

# 4. Copy source code
COPY lib/ ./lib/
COPY app-gateway/ ./app-gateway/

# 5. Generate protobufs
RUN if [ -f lib/protos/build.sh ]; then \
        echo "Found build.sh file, executing..."; \
        cd lib/protos && chmod +x ./build.sh && bash ./build.sh; \
    else \
        echo "No build.sh file found..."; \
        exit 1; \
    fi

WORKDIR /app-gateway

RUN mockery

WORKDIR /src

# 6. Download dependencies
RUN cd app-gateway && go mod download && go mod verify

RUN cd app-gateway && go tool templ generate ./...
RUN cd app-gateway && go test ./... -coverprofile=coverage.out && go vet ./...

# 7. Build - Using amd64 to match your working Users service
RUN cd app-gateway && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -v -o /gateway-exe .

# --- STAGE 2: Final Image ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates gcompat

# 1. Copy the binary
COPY --from=builder /gateway-exe /gateway-exe
COPY --from=builder /src/tailwindcss /tailwindcss

# 2. CRITICAL: Copy your static assets (HTML/CSS/JS)
# Assumes your 'public' folder is inside 'app-gateway'
COPY --from=builder /src/app-gateway/public /public

# 2. Permissions
RUN chmod +x /gateway-exe

WORKDIR /

# 3. Entrypoint
ENTRYPOINT ["/gateway-exe"]
