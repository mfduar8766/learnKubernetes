# --- STAGE 1: YOU MUST HAVE THIS AT THE TOP FOR THE COPY TO WORK ---
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /init-exe ./mqttInitContainer

# --- STAGE 2: YOUR ORIGINAL CODE ---
FROM golang:1.26.2-alpine

# Use an ARG to break the cache if needed
ARG CACHEBUST=1

# Alpine uses 'apk' instead of 'apt-get'
RUN apk update && apk add --no-cache \
    mosquitto \
    && rm -rf /var/cache/apk/*

# This line was failing because it didn't have the 'builder' stage above it
COPY --from=builder /init-exe /usr/local/bin/init-exe

# In Alpine, mosquitto_passwd is usually at /usr/bin/mosquitto_passwd automatically
# But we'll keep your symlink logic just in case
RUN BIN_PATH=$(which mosquitto_passwd) && ln -s $BIN_PATH /usr/bin/mosquitto_passwd || true

# Verification
RUN ls -l /usr/bin/mosquitto_passwd

CMD ["/usr/local/bin/init-exe"]
