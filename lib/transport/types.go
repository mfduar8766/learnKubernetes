package transport

import (
	"context"

	"github.com/eclipse/paho.golang/paho"
	protos "github.com/mfduar8766/learnKubernetes/lib/protos/generated"
)

type ITransport interface {
	Connect(ctx context.Context, clientID string, tls bool) error
	Client() *paho.Client
	IsConnectted() bool
	BuildTopic(topicType TOPIC_TYPE, domains ...string) string
	GetTopicType(topic string) *TopicProperties
	Close() error
	SubscribeMultiple(topics ...string)
	Subscribe(ctx context.Context, options []paho.SubscribeOptions) error
	UnsubscribeMultiple(topics ...string)
	// Unsubscribes from the topics and called UnregisterHandler for each topic.
	//
	// Returns a slice of errors corresponding to each topic taht was not unsubscribed.
	Unsubscribe(ctx context.Context, topics ...string) []string
	RegisterHandler(topic string, h paho.MessageHandler)
	UnregisterHandler(topic string)

	// Returns the correlation ID if it's a request, otherwise an empty string.
	Publish(ctx context.Context, topic string, payload []byte, properties *PublishRequest) (string, error)

	// MQTT v5 Request-Response Pattern
	//
	// Returns a read-only channel for the Response.
	PublishWithResponse(ctx context.Context, topic string, payload []byte, properties *PublishRequest) <-chan *protos.BrokerResponse
}

type TOPIC_TYPE int

const (
	API_VERSION                      = "v1"
	ONLINE                           = "online"
	OFFLINE                          = "offline"
	PAYLOAD_FORMAT_UTF_8             = 1
	PAYLOAD_FORMAT_BYTES             = 0
	TOPIC_TYPE_EVENT      TOPIC_TYPE = 0
	TOPIC_TYPE_REQUEST    TOPIC_TYPE = 1
	TOPIC_SEPERATOR                  = "/"
	TOPIC_WILDCARD_SINGLE            = "+"
	TOPIC_WILDCARD_MULTI             = "#"
	SERVICE_NAME                     = "serviceName"
	QoS0                             = 0
	QoS1                             = 1
	QoS2                             = 2
	DEFAULT_QoS                      = QoS1
	SERVICE_STATUS                   = "serviceStatus"

	topicEvent   = "event"
	topicRequest = "request"
	to           = "to"
	from         = "from"
)

var (
	// Default values for MQTT v5 Publish properties 30 seconds
	MESSAGE_EXPIRY = uint32(30)
	// Default session expiry interval for MQTT v5 is 0, which means the session never expires.
	// Setting it to 60 seconds for better resource management.
	SESSION_EXPIRY = uint32(60)
	// Default keep alive interval for MQTT connections in seconds. Set to 60 seconds.
	KEEP_ALIVE uint16 = 60
	// Default payload format for MQTT v5 Publish properties. Set to bytes.
	DEFAULT_PAYLOAD_FORMAT = paho.Byte(PAYLOAD_FORMAT_BYTES)
	// The time, in seconds, the broker waits before publishing the client’s LWT set to 50s
	WILL_DELAY_INTERVAL = uint32(50)
	/*
	   TopicAliasMaximum defines the maximum integer value the client accepts
	   for Topic Aliases, which are used to reduce bandwidth on repetitive publishes.
	*/
	MAX_TOPIC_ALIAS = uint16(10)
)

type TopicProperties struct {
	TopicType TOPIC_TYPE
	Domains   []string
	Topic     string
}

type ServiceStatus struct {
	Event  string `json:"event"`
	Status string `json:"status"`
}

type PublishProperties struct {
	TimeOutMS              uint32
	CorrelationData        []byte
	ContentType            string
	ResponseTopic          string
	PayloadFormat          *byte
	MessageExpiry          *uint32
	SubscriptionIdentifier *int
	TopicAlias             *uint16
	User                   paho.UserProperties
}

type PublishRequest struct {
	QoS        byte
	Retain     bool
	Topic      string
	Properties *PublishProperties
}

type SubscribeProperties struct {
	QoS                    byte
	RetainHandling         byte
	NoLocal                bool
	RetainAsPublished      bool
	SubscriptionIdentifier *int
	User                   paho.UserProperties
}

type ConnectProperties struct {
	/*
	   AuthData is the binary data associated with the AuthMethod.
	   Used for multi-step or challenge-response authentication mechanisms.
	*/
	AuthData []byte

	/*
	   AuthMethod specifies the name of the extended authentication method
	   being used (e.g., "GS2-KRB5" or a custom token exchange protocol).
	*/
	AuthMethod string

	/*
	   SessionExpiryInterval is the time in seconds that the broker keeps the
	   session alive after a disconnect. If nil or 0, the session ends immediately.
	*/
	SessionExpiryInterval *uint32

	/*
	   WillDelayInterval is the time in seconds the broker waits before publishing
	   the client's Will Message, preventing spam during brief network drops.
	*/
	WillDelayInterval *uint32

	/*
	   ReceiveMaximum limits the number of concurrent, unacknowledged QoS 1 and QoS 2
	   messages the client is willing to process at one time (in-flight limit).
	*/
	ReceiveMaximum *uint16

	/*
	   TopicAliasMaximum defines the maximum integer value the client accepts
	   for Topic Aliases, which are used to reduce bandwidth on repetitive publishes.
	*/
	TopicAliasMaximum *uint16

	/*
	   MaximumPacketSize defines the maximum total packet size (in bytes) that
	   the client is willing to accept from the broker.
	*/
	MaximumPacketSize *uint32

	/*
	   User contains custom key-value string pairs (metadata), acting like
	   custom HTTP headers sent during the connection handshake.
	*/
	User paho.UserProperties

	/*
	   RequestProblemInfo, when true, asks the broker to return human-readable
	   Reason Strings or User Properties in error scenarios (CONNACK / DISCONNECT).
	*/
	RequestProblemInfo bool

	/*
	   RequestResponseInfo, when true, asks the broker to return a Response Information
	   string in the CONNACK, typically used to set up Request-Response topologies.
	*/
	RequestResponseInfo bool
}
