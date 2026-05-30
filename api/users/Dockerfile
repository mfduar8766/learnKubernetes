# --- STAGE 1: Build ---
FROM golang:1.26.2-alpine AS builder

WORKDIR /src

# 1. Copy manifests for both lib and the users service to cache layers
COPY lib/go.mod lib/go.sum ./lib/
COPY api/users/go.mod api/users/go.sum ./api/users/

# 2. Download dependencies
# This resolves your internal 'lib' dependency via the 'replace' in go.mod
RUN cd api/users && go mod download && go mod verify

# 3. Copy the source code for the required modules
COPY lib/ ./lib/
COPY api/users/ ./api/users/

# 4. Protobuf Generation
RUN apk add --no-cache protoc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# RUN chmod +x ./lib/protos/build.sh
# RUN ./lib/protos/build.sh

# 5. Run quality checks
RUN cd api/users && go test ./... && go vet ./...

# 6. Build the statically linked binary
# We output it to a unique name: users-exe
# RUN cd api/users && CGO_ENABLED=0 go build -v -o /users-exe .
RUN cd api/users && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /users-exe .

# --- STAGE 2: Final Image ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# 1. Copy the binary
COPY --from=builder /users-exe /users-exe

# 2. Explicitly set permissions so it's not left to chance
RUN chmod +x /users-exe

WORKDIR /

# 3. Use the absolute path to avoid any ambiguity
ENTRYPOINT ["/users-exe"]

# # --- STAGE 2: Final Image ---
# FROM alpine:latest

# # Basic security certificates
# RUN apk --no-cache add ca-certificates

# # Copy only the final binary
# COPY --from=builder /users-exe /users-exe

# WORKDIR /

# # Run the users service
# CMD ["./users-exe"]
