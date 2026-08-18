# --- STAGE 1: Build ---
FROM golang:1.26.2-alpine AS builder

WORKDIR /src

ARG TAIL_WIND_VRSION=4.2.2

# 1. Install dependencies
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

# 3. Copy manifests for both lib and the users service to cache layers
COPY lib/go.mod lib/go.sum ./lib/
COPY api/users/go.mod api/users/go.sum ./api/users/

# 4. Copy the source code for the required modules
COPY lib/ ./lib/
COPY api/users/ ./api/users/

WORKDIR /src/lib

# 5. Generate protobufs
RUN if [ -f /src/lib/protos/build.sh ]; then \
        echo "Found build.sh file, executing..."; \
        cd /src/lib/protos && chmod +x ./build.sh && bash ./build.sh; \
    else \
        echo "No build.sh file found..."; \
        exit 1; \
    fi

RUN mockery

WORKDIR /api/users
RUN mockery

# 6. Download dependencies
# This resolves your internal 'lib' dependency via the 'replace' in go.mod
RUN go mod download && go mod verify

# 7. Run quality checks
RUN go test ./... -coverprofile=coverage.out && go vet ./...

# 7. Build the statically linked binary
# We output it to a unique name: users-exe
# RUN cd api/users && CGO_ENABLED=0 go build -v -o /users-exe .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /users-exe .

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
