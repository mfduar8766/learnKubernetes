package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	// rv8 "github.com/go-redis/redis/v8"

	"github.com/eclipse/paho.golang/paho"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/transport"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	mqtt "github.com/mochi-mqtt/server/v2"
	"gopkg.in/yaml.v2"

	// "github.com/mochi-mqtt/server/v2/hooks/storage/redis"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

type TelemetryData struct {
	Temperature float64 `json:"temperature"`
	Status      string  `json:"status"`
}

type BaseConfig struct {
	Servers []ServerConfig `yaml:"server"`
	// acl_rules is now an object, not an array
	AclRules AclRulesConfig `yaml:"acl_rules"`
}

type ServerConfig struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Port     int      `yaml:"port"`
	Protocol string   `yaml:"protocol"`
	Hooks    []string `yaml:"hooks"`
}

// AclRulesConfig wraps the new "allowed_topics" nesting layer
type AclRulesConfig struct {
	AllowedTopics []AclRuleConfig `yaml:"allowed_topics"`
}

type AclRuleConfig struct {
	Topic                  string `yaml:"topic"`
	AllowPublish           bool   `yaml:"allow_publish"`
	AllowSubscribe         bool   `yaml:"allow_subscribe"`
	TopicAlias             uint16 `yaml:"topic_alias"`
	Qos                    byte   `yaml:"qos"`
	SubscriptionIdentifier int    `yaml:"subscription_identifier"`
	Retain                 bool   `yaml:"retain"`
}

type Proxy struct {
	mqtt.HookBase
	Ctx            context.Context
	BackendClients map[string]transport.ITransport
	TopicsMap      map[string]*AclRuleConfig
	TopicMapMutex  sync.RWMutex
	Logger         logger.ILogger
	Config         *BaseConfig
	// Map of all the Mqtt available hooks
	// and their corresponding byte values for
	// quick lookup when registering hooks based on config
	HooksMap        map[string]byte
	BackEndMapMutex sync.RWMutex
}

func New(ctx context.Context, logger logger.ILogger) *Proxy {
	var proxy = Proxy{
		Ctx:            ctx,
		BackendClients: make(map[string]transport.ITransport),
		TopicsMap:      make(map[string]*AclRuleConfig),
		Logger:         logger,
		Config: &BaseConfig{
			Servers: []ServerConfig{
				{
					Type:     "tcp",
					Name:     "test",
					Port:     1883,
					Protocol: "mqtt",
					Hooks: []string{
						"OnPacketRead",
						"OnACLCheck",
						"OnConnect",
						"OnConnectAuthenticate",
						"OnSubscribe",
						"OnSubscribed",
						"OnPublish",
						"OnPublished",
						"OnDisconnect",
					},
				},
			},
			AclRules: AclRulesConfig{
				AllowedTopics: []AclRuleConfig{
					{
						Topic:                  "app/v1/client/+/test",
						AllowPublish:           true,
						AllowSubscribe:         true,
						Qos:                    1,
						SubscriptionIdentifier: 1,
						TopicAlias:             1,
						Retain:                 false,
					},
				},
			},
		},
		HooksMap: map[string]byte{
			// Server Lifecycle Hooks
			"OnStarted": mqtt.OnStarted,
			"OnStopped": mqtt.OnStopped,

			// Connection & Session Hooks
			"OnConnectAuthenticate": mqtt.OnConnectAuthenticate,
			"OnACLCheck":            mqtt.OnACLCheck,
			"OnConnect":             mqtt.OnConnect,
			"OnSessionEstablish":    mqtt.OnSessionEstablish,
			"OnSessionEstablished":  mqtt.OnSessionEstablished,
			"OnDisconnect":          mqtt.OnDisconnect,
			"OnAuthPacket":          mqtt.OnAuthPacket,

			// Low-Level Packet Iteration Hooks
			"OnPacketRead":      mqtt.OnPacketRead,
			"OnPacketEncode":    mqtt.OnPacketEncode,
			"OnPacketSent":      mqtt.OnPacketSent,
			"OnPacketProcessed": mqtt.OnPacketProcessed,

			// Subscription Pipeline Hooks
			"OnSubscribe":         mqtt.OnSubscribe,
			"OnSubscribed":        mqtt.OnSubscribed,
			"OnSelectSubscribers": mqtt.OnSelectSubscribers,
			"OnUnsubscribe":       mqtt.OnUnsubscribe,
			"OnUnsubscribed":      mqtt.OnUnsubscribed,

			// Publishing Lifecycle Hooks
			"OnPublish":        mqtt.OnPublish,
			"OnPublished":      mqtt.OnPublished,
			"OnPublishDropped": mqtt.OnPublishDropped,

			// Retain State Hooks
			"OnRetainMessage":   mqtt.OnRetainMessage,
			"OnRetainPublished": mqtt.OnRetainPublished,
			"OnRetainedExpired": mqtt.OnRetainedExpired,

			// QoS Handshake Handling Hooks
			"OnQosPublish":  mqtt.OnQosPublish,
			"OnQosComplete": mqtt.OnQosComplete,
			"OnQosDropped":  mqtt.OnQosDropped,

			// Miscellaneous Edge Case Hooks
			"OnSysInfoTick":       mqtt.OnSysInfoTick,
			"OnPacketIDExhausted": mqtt.OnPacketIDExhausted,
			"OnWill":              mqtt.OnWill,
			"OnWillSent":          mqtt.OnWillSent,
			"OnClientExpired":     mqtt.OnClientExpired,

			// Storage Backend/Persistence Hooks
			"StoredClients":          mqtt.StoredClients,
			"StoredSubscriptions":    mqtt.StoredSubscriptions,
			"StoredInflightMessages": mqtt.StoredInflightMessages,
			"StoredRetainedMessages": mqtt.StoredRetainedMessages,
			"StoredSysInfo":          mqtt.StoredSysInfo,
		},
	}
	proxy.parseConfig("config.yaml")
	return &proxy
}

func (p *Proxy) parseConfig(configPath string) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		p.Logger.LogErrorf("Failed to read config file using default configs instead: %v", err)
		return
	}
	var config BaseConfig
	if err := yaml.Unmarshal(file, &config); err != nil {
		p.Logger.LogErrorf("Failed to parse config file using default configs instead: %v", err)
		return
	}
	p.Config = &config
}

func (p *Proxy) Connect(clients map[string]string) error {
	if len(clients) == 0 {
		return fmt.Errorf("no backend clients provided for proxy")
	}
	for client, host := range clients {
		p.Logger.LogInfof("Connecting to backend broker for client '%s' at '%s'", client, host)
		clientTransport := transport.New(p.Logger)
		if err := clientTransport.Connect(p.Ctx, host, false); err != nil {
			p.Logger.LogErrorf("Failed to connect to backend broker for client '%s': %v", client, err)
			return err
		}
		p.BackendClients[client] = clientTransport
	}
	return nil
}

func (p *Proxy) matchTopic(topic string, pattern string) bool {
	topicParts := strings.Split(topic, transport.TOPIC_SEPERATOR)
	patternParts := strings.Split(pattern, transport.TOPIC_SEPERATOR)

	for i, patternPart := range patternParts {
		if patternPart == "+" {
			continue
		}
		if patternPart == "#" {
			return true
		}
		if i >= len(topicParts) || topicParts[i] != patternPart {
			return false
		}
	}
	return len(topicParts) == len(patternParts)
}

func (p *Proxy) Provides(b byte) bool {
	for _, s := range p.Config.Servers {
		for _, hook := range s.Hooks {
			if p.HooksMap[hook] == b {
				return true
			}
		}
	}
	return false
}

func (p *Proxy) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	p.TopicMapMutex.RLock()
	topicData, exists := p.TopicsMap[topic]
	p.TopicMapMutex.RUnlock()
	if exists {
		if write {
			return topicData.AllowPublish
		}
		return topicData.AllowSubscribe
	}
	for _, rule := range p.Config.AclRules.AllowedTopics {
		if topic == rule.Topic || p.matchTopic(topic, rule.Topic) {
			p.TopicMapMutex.Lock()
			p.TopicsMap[topic] = &AclRuleConfig{
				AllowPublish:           rule.AllowPublish,
				AllowSubscribe:         rule.AllowSubscribe,
				Qos:                    rule.Qos,
				TopicAlias:             rule.TopicAlias,
				SubscriptionIdentifier: rule.SubscriptionIdentifier,
				Retain:                 rule.Retain,
			}
			p.TopicMapMutex.Unlock()
			if write {
				return rule.AllowPublish
			}
			return rule.AllowSubscribe
		}
	}
	log.Printf("[ACL NO MATCH] Client '%s' topic '%s' did not match any ACL rules. Denying by default.", cl.ID, topic)
	return false
}

func (h *Proxy) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	fmt.Printf("OnPacketRead():: Client '%s' sent packet on topic '%s' payload: %s\n", cl.ID, pk.TopicName, pk.Payload)
	// if pk.FixedHeader.Type == packets.Subscribe {
	// 	for _, filter := range pk.Filters {
	// 		if filter.Filter == "#" {
	// 			return pk, packets.ErrRejectPacket
	// 		}
	// 	}
	// }
	// switch pk.FixedHeader.Type {
	// case packets.Publish:
	// 	if pk.TopicName == "forbidden/topic" {
	// 		log.Printf("[PROXY] Blocking publish to '%s' from client '%s'", pk.TopicName, cl.ID)
	// 		return pk, packets.ErrRejectPacket
	// 	}
	// case packets.Connect:
	// 	if string(pk.Connect.Username) != "user" || string(pk.Connect.Password) != "password" {
	// 		log.Printf("OnPacketRead()::Authentication failed for client '%s'", cl.ID)
	// 		return pk, packets.ErrRejectPacket
	// 	}
	// 	log.Printf("OnPacketRead()::Authentication successful for client '%s'", cl.ID)
	// 	return pk, nil
	// }
	return pk, nil
}

// func (h *Proxy) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
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

func (h *Proxy) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	fmt.Printf("Mochi Intercepted Connect :: clientID: %s\n", cl.ID)
	return nil
	// var targetBroker string
	// switch cl.ID {
	// case CLIENT1:
	// 	targetBroker = "127.0.0.1:1884" // Port-forwarded Mosquitto 1
	// case CLIENT2:
	// 	targetBroker = "127.0.0.1:1885" // Port-forwarded Mosquitto 2
	// default:
	// 	// Unknown client, let it connect to Mochi or reject it
	// 	return nil
	// }

	// // Build the MQTT v5 Server Moved redirection packet
	// connAck := packets.Packet{
	// 	FixedHeader: packets.FixedHeader{
	// 		Type: packets.Connack,
	// 	},
	// 	ReasonCode: packets.ErrServerMoved.Code, // 0x9D
	// 	Properties: packets.Properties{
	// 		ServerReference: targetBroker,
	// 	},
	// }

	// // Write the redirection instructions back to the client
	// if err := cl.WritePacket(connAck); err != nil {
	// 	log.Printf("Failed to send redirection packet to client '%s': %v", cl.ID, err)
	// 	return err
	// }

	// // Returning an error breaks the temporary socket with Mochi gracefully
	// return packets.ErrServerMoved
}

func (h *Proxy) OnAuthPacket(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	return pk, nil
}

func (h *Proxy) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)
	if username == "user" && password == "password" {
		log.Printf("[AUTH SUCCESS] Client '%s' approved.", cl.ID)
		return true
	}
	return false
}

func (p *Proxy) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if pk.FixedHeader.Type != packets.Publish {
		return pk, packets.ErrMalformedPacket
	}

	log.Printf("[PROXY] Intercepted publish on '%s' from '%s'", pk.TopicName, cl.ID)
	var telemetry TelemetryData
	err := json.Unmarshal(pk.Payload, &telemetry)
	if err != nil || telemetry.Temperature > 100.0 {
		log.Println("[VALIDATION FAILED] Dropping packet.")
		return pk, packets.ErrRejectPacket
	}

	p.TopicMapMutex.RLock()
	topicData, exists := p.TopicsMap[pk.TopicName]
	p.TopicMapMutex.RUnlock()

	if exists {
		ctx, cancel := context.WithTimeout(p.Ctx, 1000*time.Millisecond)
		defer cancel()

		_, err := p.BackendClients[cl.ID].Publish(ctx, pk.TopicName, pk.Payload, &transport.PublishRequest{
			QoS:    topicData.Qos,
			Retain: topicData.Retain,
			Properties: &transport.PublishProperties{
				ContentType:   types.HEADER_APPLICATION_JSON,
				PayloadFormat: transport.DEFAULT_PAYLOAD_FORMAT,
				TopicAlias:    &topicData.TopicAlias,
				// User:                   pk.Properties.User,
				SubscriptionIdentifier: &topicData.SubscriptionIdentifier,
			},
		})
		if err != nil {
			p.Logger.LogErrorf("Proxy::OnPublish()::Publish()::received error: %+v", err)
			return pk, packets.ErrRejectPacket
		}
	}
	return pk, packets.Err3ClientIdentifierNotValid
}

func (h *Proxy) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	fmt.Printf("Proxy::OnPublished()::ClientID: %s, Client '%s' sent packet on topic '%s' payload: %s\n", pk.Origin, cl.ID, pk.TopicName, pk.Payload)
}

func (p *Proxy) OnSubscribe(cl *mqtt.Client, pk packets.Packet) packets.Packet {
	if pk.FixedHeader.Type != packets.Subscribe {
		p.Logger.LogWarnf("Proxy::OnSubscribe()::incoming packet is not sub packet...")
		return pk
	}

	p.BackEndMapMutex.RLock()
	broker, exists := p.BackendClients[cl.ID]
	p.BackEndMapMutex.RUnlock()
	if !exists {
		return pk
	}

	var filters []paho.SubscribeOptions
	p.TopicMapMutex.RLock()
	for _, filter := range pk.Filters {
		topicName := filter.Filter
		topicData, exists := p.TopicsMap[topicName]
		if exists && topicData.AllowSubscribe {
			filters = append(filters, paho.SubscribeOptions{
				Topic: topicName,
				QoS:   topicData.Qos,
			})
		} else {
			allowedByConfig := false
			for _, rule := range p.Config.AclRules.AllowedTopics {
				if topicName == rule.Topic || p.matchTopic(topicName, rule.Topic) {
					if rule.AllowSubscribe {
						allowedByConfig = true
						filters = append(filters, paho.SubscribeOptions{
							Topic: topicName,
							QoS:   topicData.Qos,
						})
					}
					break
				}
			}
			if !allowedByConfig {
				p.Logger.LogWarnf("Proxy::OnSubscribe()::topic: %s subscription is not allowed...", topicName)
			}
		}
	}
	p.TopicMapMutex.RUnlock()

	if len(filters) > 0 {
		go func(subOptions []paho.SubscribeOptions) {
			ctx, cancel := context.WithTimeout(p.Ctx, 1000*time.Millisecond)
			defer cancel()

			err := broker.Subscribe(ctx, subOptions)
			if err != nil {
				p.Logger.LogErrorf("Proxy::Subscribe()::received error: %+v", err)
				return
			}
		}(filters)
	}

	return pk
}

func (p *Proxy) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	p.Logger.LogInfof("Proxy::OnSubscribed()::clientID: %s sucscribed to topic: %s", cl.ID, pk.TopicName)
}

func (h *Proxy) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	log.Printf("[PROXY] Client '%s' disconnected. Reason: %v", cl.ID, err)
}

func main() {
	logger := logger.NewLogger("proxy")
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	/*
		1. Initialize the backend transport client to connect to your core enterprise broker.

		Client1 and client2 are 2 independent deployments of Eclipse Mosquitto running in the Kubernetes Cluster.

		When running from source mqtt-deployment (client1) and mqtt2-deployment (client2) are port forwarded on port 1883.
	*/
	var clientMap map[string]string = map[string]string{
		"mqtt-service:1883":  "mqtt-service:1883",
		"mqtt2-service:1883": "mqtt2-service:1883",
	}
	server := mqtt.New(nil)
	proxy := New(ctx, logger)
	if err := proxy.Connect(clientMap); err != nil {
		logger.LogFatalf("Failed to connect proxy to backend brokers: %v", err)
		return
	}

	if err := server.AddHook(proxy, nil); err != nil {
		logger.LogFatalf("Failed to add proxy hook to server: %v", err)
		return
	}

	// Run Mochi on port 1884 so it acts as your edge gateway proxy
	tcpListener := listeners.NewTCP(listeners.Config{Address: ":1883"})
	if err := server.AddListener(tcpListener); err != nil {
		logger.LogFatalf("Failed to add TCP listener: %v", err)
		return
	}

	go func() {
		if err := server.Serve(); err != nil {
			logger.LogFatalf("Server error: %v", err)
			return
		}
	}()

	log.Println("Mochi edge gateway actively listening on port 1883...")
	<-ctx.Done()

	log.Println("Shutting down Mochi edge gateway...")

	if err := server.Close(); err != nil {
		logger.LogErrorf("Error during server shutdown: %v", err)
		return
	}
}
