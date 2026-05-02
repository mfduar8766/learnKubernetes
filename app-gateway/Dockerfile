FROM golang:1.26.2-alpine AS development

WORKDIR /usr/src/app

# 1. Pre-copy/cache dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# 2. Copy source code
# Make sure you ran 'templ generate' on your host machine before this!
COPY . .

RUN go vet ./...

RUN go test ./...

# If we dont want to build it from src and just do it in the dockerfile do this
# RUN go install github.com/a-h/templ/cmd/templ@latest
# RUN templ generate

# Build the binary into the current WORKDIR (/usr/src/app)
RUN CGO_ENABLED=0 go build -v -o gateway-exe .

# Run from /usr/src/app so it can see the ./public folder
CMD ["./gateway-exe"]
