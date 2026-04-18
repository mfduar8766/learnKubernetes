package transport

import (
	"context"

	"github.com/eclipse/paho.golang/paho"
)

type ITransport interface {
	Connect(ctx context.Context, clientID string, tls bool) error
	BuildTopic(topicType TOPIC_TYPE, domains ...string) string
	GetTopicType(topic string) *TopicProperties
	Close() error
	Subscribe(ctx context.Context, topic string, properties *SubscribeProperties) error

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
	PublishWithResponse(ctx context.Context, topic string, payload []byte, properties *PublishRequest) <-chan Response
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

	topicEvent   = "event"
	topicRequest = "request"
	to           = "to"
	from         = "from"
)

var (
	MESSAGE_EXPIRY        = new(uint32(30))
	SESSION_EXPIRY        = new(uint32(60))
	KEEP_ALIVE     uint16 = 60
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

type Response struct {
	Topic         string
	ResponseTopic string
	CorrelationID string
	Payload       []byte
	Error         error
	TimeOut       bool
}
