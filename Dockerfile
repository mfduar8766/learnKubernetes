FROM golang:latest AS development

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

# 3. Build with CGO disabled for a "static" binary 
# (This prevents 'file not found' errors in K8s)
# RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/app .

# # 4. Set the binary to run
# CMD ["/usr/local/bin/app"]

RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/gateway-exe .

CMD ["/usr/local/bin/gateway-exe"]
