package rmq

import (
	"context"
	"sync"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

type IRabbitMq interface {
	NewRmq(ctx context.Context, log *logger.Logger, appId string) *RMQ
	Consume(topic string, isPublish bool, response []byte) *models.MessagePayload
	Subscribe(topic string)
	Publish(topic string, message []byte)
	PubSub(topic string, message []byte) (*models.MessagePayload, error)
	ClearSubscriptions()
}

type TopicTypes string

const (
	Request  TopicTypes = "request"
	Response TopicTypes = "response"
	Events   TopicTypes = "events"
)

// Queues
const (
	USERS_QUEUE = "users_queue"
	POSTS_QUEUE = "posts_queue"
)

// Exchanges
const (
	USERS_EX = "users_ex"
	POSTS_EX = "posts_ex"
)

// Exchanges
const (
	DEFAULT = amqp.DefaultExchange
	DIRECT  = amqp.ExchangeDirect
	FANOUT  = amqp.ExchangeFanout
	TOPIC   = amqp.ExchangeTopic
	HEADERS = amqp.ExchangeHeaders
)

type RMQ struct {
	log            *logger.Logger
	Connection     *amqp.Connection
	Channel        *amqp.Channel
	Ctx            context.Context
	appID          string
	subscribers    map[string]subscriber
	mutx           Mutexs
	terminatedChan chan bool
}

type RmqPubSubResponse struct {
	Service string
	Payload []byte
}

type subscriber struct {
	topic            string
	subscriptionType string
	isAck            bool
	appId            string
	queueName        string
	exchangeName     string
	exchangeType     string
	id               string
}

type Mutexs struct {
	pubSubMutex    *sync.Mutex
	pubMutex       *sync.Mutex
	consumeMutex   *sync.Mutex
	subscribeMutex *sync.Mutex
}
