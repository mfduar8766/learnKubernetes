FROM golang:latest AS development

WORKDIR /usr/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go vet ./...

RUN go test ./...

# 1. Use a UNIQUE name for the output
# 2. Point EXACTLY to the users main.go
RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/users-exe ./api/users

CMD ["/usr/local/bin/users-exe"]
