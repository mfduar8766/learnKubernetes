# --- STAGE 1: Build ---
FROM golang:1.26.2-alpine AS builder

WORKDIR /src

ARG TAIL_WIND_VRSION=4.2.2

# 1. Install dependencies
ADD https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAIL_WIND_VRSION}/tailwindcss-linux-x64 ./tailwindcss
RUN chmod +x ./tailwindcss
RUN apk add --no-cache protoc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN go install github.com/a-h/templ/cmd/templ@latest

# 2. Copy manifests
COPY lib/go.mod lib/go.sum ./lib/
COPY app-gateway/go.mod app-gateway/go.sum ./app-gateway/

# 3. Copy source code
COPY lib/ ./lib/
COPY app-gateway/ ./app-gateway/

# 4. Generate protobufs
RUN chmod +x ./lib/protos/build.sh
RUN ./lib/protos/build.sh

# 5. Download dependencies
RUN cd app-gateway && go mod download && go mod verify

RUN cd app-gateway && go tool templ generate ./...
RUN cd app-gateway && go test ./... && go vet ./...

# 6. Build - Using amd64 to match your working Users service
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
