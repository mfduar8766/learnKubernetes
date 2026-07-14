module github.com/mfduar8766/learnKubernetes/broker

go 1.26.2

require (
	github.com/eclipse/paho.golang v0.23.0
	github.com/mfduar8766/learnKubernetes/lib v0.0.0
	github.com/mochi-mqtt/server/v2 v2.7.9
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/caarlos0/env/v11 v11.4.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/rs/xid v1.4.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/mfduar8766/learnKubernetes/lib => ../lib
