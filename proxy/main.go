package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	// rv8 "github.com/go-redis/redis/v8"
	"github.com/eclipse/paho.golang/paho"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/transport"
	mqtt "github.com/mochi-mqtt/server/v2"

	// "github.com/mochi-mqtt/server/v2/hooks/storage/redis"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

const (
	CLIENT1 = "client1"
	CLIENT2 = "client2"
)

type TelemetryData struct {
	Temperature float64 `json:"temperature"`
	Status      string  `json:"status"`
}

type Broker struct {
	Name   string
	Url    string
	Client *paho.Client
}

type ManagedProxyHook struct {
	mqtt.HookBase
	BackendClients map[string]*Broker // Client connection to your backend cluster
	Routes         map[string]string
	routeMutex     sync.RWMutex
}

func New(clientMap map[string]string, logger logger.ILogger) (*ManagedProxyHook, error) {
	var clients map[string]*Broker = make(map[string]*Broker, len(clientMap))
	for name, host := range clientMap {
		client := transport.New(logger)
		if err := client.Connect(context.Background(), name, false); err != nil {
			return nil, err
		}
		clients[name] = &Broker{
			Url:    host,
			Client: client.Client(),
		}
	}
	// Pass the backend client reference into our validation hook
	return &ManagedProxyHook{
		BackendClients: clients,
		Routes: map[string]string{
			CLIENT1: CLIENT1,
			CLIENT2: CLIENT2,
		},
		routeMutex: sync.RWMutex{},
	}, nil
}

func (h *ManagedProxyHook) ID() string {
	return "mochi-backend-bridge"
}

func (h *ManagedProxyHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnPacketRead,
		mqtt.OnAuthPacket,
		mqtt.OnACLCheck,
		mqtt.OnConnect,
		mqtt.OnConnectAuthenticate,
		mqtt.OnPublished,
		mqtt.OnPublish,
		mqtt.OnDisconnect,
	}, []byte{b})
}

func (h *ManagedProxyHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	// log.Printf("[ACL CHECK] Client '%s' requesting topic '%s' (Write: %v)", cl.ID, topic, write)
	return true
}

// STAGE 1 & 2: Structural and Authentication filters (same as before)
func (h *ManagedProxyHook) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	fmt.Printf("OnPacketRead():: Client '%s' sent packet on topic '%s' payload: %s\n", cl.ID, pk.TopicName, pk.Payload)
	// if pk.FixedHeader.Type == packets.Subscribe {
	// 	for _, filter := range pk.Filters {
	// 		if filter.Filter == "#" {
	// 			return pk, packets.ErrRejectPacket
	// 		}
	// 	}
	// }
	switch pk.FixedHeader.Type {
	case packets.Publish:
		if pk.TopicName == "forbidden/topic" {
			log.Printf("[PROXY] Blocking publish to '%s' from client '%s'", pk.TopicName, cl.ID)
			return pk, packets.ErrRejectPacket
		}
	case packets.Connect:
		if string(pk.Connect.Username) != "user" || string(pk.Connect.Password) != "password" {
			log.Printf("OnPacketRead()::Authentication failed for client '%s'", cl.ID)
			return pk, packets.ErrRejectPacket
		}
		log.Printf("OnPacketRead()::Authentication successful for client '%s'", cl.ID)
		return pk, nil
	}
	return pk, nil
}

// func (h *ManagedProxyHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
// 	fmt.Printf("OnConnect()::clientID: %s params: %+v\n", cl.ID, pk.Connect)
// 	if cl.ID == CLIENT1 {
// 		connAck := packets.Packet{
// 			FixedHeader: packets.FixedHeader{
// 				Type: packets.Connack,
// 			},
// 			ReasonCode: packets.ErrServerMoved.Code,
// 			Properties: packets.Properties{
// 				ServerReference: "127.0.0.1:1885",
// 			},
// 		}
// 		if err := cl.WritePacket(connAck); err != nil {
// 			return err
// 		}
// 		return packets.ErrServerMoved
// 	}
// 	return nil
// }

func (h *ManagedProxyHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	fmt.Printf("Mochi Intercepted Connect :: clientID: %s\n", cl.ID)

	var targetBroker string
	switch cl.ID {
	case CLIENT1:
		targetBroker = "127.0.0.1:1884" // Port-forwarded Mosquitto 1
	case CLIENT2:
		targetBroker = "127.0.0.1:1885" // Port-forwarded Mosquitto 2
	default:
		// Unknown client, let it connect to Mochi or reject it
		return nil
	}

	// Build the MQTT v5 Server Moved redirection packet
	connAck := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connack,
		},
		ReasonCode: packets.ErrServerMoved.Code, // 0x9D
		Properties: packets.Properties{
			ServerReference: targetBroker,
		},
	}

	// Write the redirection instructions back to the client
	if err := cl.WritePacket(connAck); err != nil {
		log.Printf("Failed to send redirection packet to client '%s': %v", cl.ID, err)
		return err
	}

	// Returning an error breaks the temporary socket with Mochi gracefully
	return packets.ErrServerMoved
}

func (h *ManagedProxyHook) OnAuthPacket(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	fmt.Printf("OnAuthPacket():: Client '%s' sent auth packet with payload: %s\n", cl.ID, pk.Payload)
	return pk, nil
}

func (h *ManagedProxyHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)
	fmt.Printf("OnConnectAuthenticate()::VERSION: %s PROTO: %s USER: %s PASS: %s\n", string(pk.ProtocolVersion), pk.Connect.ProtocolName, username, password)

	if username == "user" && password == "password" {
		log.Printf("[AUTH SUCCESS] Client '%s' approved.", cl.ID)
		return true
	}

	log.Printf("[AUTH FAILED] Client '%s' failed credentials matching.", cl.ID)
	return false
}

func (h *ManagedProxyHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	fmt.Printf("OnPublished():: Origin: %s, Client '%s' sent packet on topic '%s' payload: %s\n", pk.Origin, cl.ID, pk.TopicName, pk.Payload)
}

// STAGE 3: Validate, and explicitly FORWARD to the backend broker
func (h *ManagedProxyHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	log.Printf("[PROXY] Intercepted publish on '%s' from '%s'", pk.TopicName, cl.ID)

	// Business rule validations
	var telemetry TelemetryData
	err := json.Unmarshal(pk.Payload, &telemetry)
	if err != nil || telemetry.Temperature > 100.0 {
		log.Println("[VALIDATION FAILED] Dropping packet.")
		return pk, packets.ErrRejectPacket
	}

	log.Println("[VALIDATION PASSED] Forwarding message to backend broker...")

	if client, exists := h.BackendClients[pk.Origin]; exists {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Push the validated packet straight to your core enterprise broker
		_, err = client.Client.Publish(ctx, &paho.Publish{
			Topic:   pk.TopicName,
			QoS:     pk.FixedHeader.Qos,
			Retain:  pk.FixedHeader.Retain,
			Payload: pk.Payload,
		})
		if err != nil {
			log.Printf("[BACKEND ERROR] Failed to deliver packet to core broker: %v", err)
			// Decide if you want to fail open or fail closed here
		}

		// Reject locally so Mochi doesn't store duplicate states in its own engine memory
		return pk, packets.ErrRejectPacket
	}
	return pk, packets.ErrClientIdentifierNotValid
}

func (h *ManagedProxyHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	log.Printf("[PROXY] Client '%s' disconnected. Reason: %v", cl.ID, err)
}

func main() {
	logger := logger.NewLogger("proxy")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	/*
		1. Initialize the backend transport client to connect to your core enterprise broker.

		Client1 and client2 are 2 independent deployments of Eclipse Mosquitto running in the Kubernetes Cluster.

		When running from source mqtt-deployment (client1) and mqtt2-deployment (client2) are port forwarded on port 1883.
	*/
	var clientMap map[string]string = map[string]string{
		CLIENT1: "mqtt-service:1883",
		CLIENT2: "mqtt2-service:1883",
	}

	// 2. Initialize the Mochi Frontend Edge Server
	server := mqtt.New(nil)

	// Pass the backend client reference into our validation hook
	proxy, err := New(clientMap, logger)
	if err != nil {
		logger.LogFatalf("Failed to initialize proxy hook: %v", err)
		return
	}

	if err = server.AddHook(proxy, nil); err != nil {
		logger.LogFatalf("Failed to add proxy hook to server: %v", err)
		return
	}

	// Run Mochi on port 1884 so it acts as your edge gateway proxy
	tcpListener := listeners.NewTCP(listeners.Config{Address: ":1883"})
	if err = server.AddListener(tcpListener); err != nil {
		logger.LogFatalf("Failed to add TCP listener: %v", err)
		os.Exit(1)
	}

	go func() {
		if err = server.Serve(); err != nil {
			logger.LogFatalf("Server error: %v", err)
			os.Exit(1)
		}
	}()

	log.Println("Mochi edge gateway actively listening on port 1883...")
	<-sigs
	server.Close()
}
