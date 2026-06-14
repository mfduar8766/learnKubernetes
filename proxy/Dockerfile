# --- STAGE 1: Build ---
FROM golang:1.26.2-alpine AS builder

WORKDIR /src

# 1. Copy manifests for both lib and the users service to cache layers
COPY lib/go.mod lib/go.sum ./lib/
COPY proxy/go.mod proxy/go.sum ./proxy/

# 2. Download dependencies
# This resolves your internal 'lib' dependency via the 'replace' in go.mod
RUN cd proxy && go mod download && go mod verify

# 3. Copy the source code for the required modules
COPY lib/ ./lib/
COPY proxy/ ./proxy/

# 4. Protobuf Generation
RUN apk add --no-cache protoc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# RUN chmod +x ./lib/protos/build.sh
# RUN ./lib/protos/build.sh

# 5. Run quality checks
RUN cd proxy && go test ./... && go vet ./...

# 6. Build the statically linked binary
# We output it to a unique name: users-exe
# RUN cd api/users && CGO_ENABLED=0 go build -v -o /users-exe .
RUN cd proxy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /proxy-exe .

# --- STAGE 2: Final Image ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# 1. Copy the binary
COPY --from=builder /proxy-exe /proxy-exe

# 2. Explicitly set permissions so it's not left to chance
RUN chmod +x /proxy-exe

WORKDIR /

# 3. Use the absolute path to avoid any ambiguity
ENTRYPOINT ["/proxy-exe"]
