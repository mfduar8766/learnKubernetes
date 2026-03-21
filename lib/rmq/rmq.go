package rmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	amqp "github.com/rabbitmq/amqp091-go"
)

func NewRmq(ctx context.Context, log *logger.Logger, appId string) *RMQ {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	rmqURL := "amqp://user:password@localhost:5672"
	if value := os.Getenv(types.RABBITMQ_URI); len(value) > 0 {
		rmqURL = value
	}
	log.LogInfof("RMQ::NewRmq():: connected to: %s", rmqURL)
	connection, err := amqp.Dial(rmqURL)
	utils.MustHandleError("NewRmq()", "Error connecting...", err, log)

	channel, err := connection.Channel()
	utils.MustHandleError("NewRmq()", "Error creating channel...", err, log)

	// CRITICAL: Request confirmations from the server
	if err := channel.Confirm(false); err != nil {
		log.LogErrorf("Could not set channel to confirm mode: %s", err)
	}

	rmq := &RMQ{
		log:            log,
		Connection:     connection,
		Channel:        channel,
		Ctx:            ctx,
		appID:          fmt.Sprintf("%s:%s", appId, uuid.New().String()),
		terminatedChan: make(chan bool, 1),
		subscribers:    make(map[string]subscriber),
		mutx: Mutexs{
			pubSubMutex:    &sync.Mutex{},
			pubMutex:       &sync.Mutex{},
			consumeMutex:   &sync.Mutex{},
			subscribeMutex: &sync.Mutex{},
		},
	}

	// Monitor for background closures
	go func() {
		closeErr := <-rmq.Channel.NotifyClose(make(chan *amqp.Error))
		if closeErr != nil {
			log.LogErrorf("RabbitMQ Channel closed unexpectedly: %v", closeErr)
			// Here is where you would trigger a reconnect logic
		}
	}()

	// Give the network a tiny "breath" to ensure the handshake is 100% complete
	time.Sleep(50 * time.Millisecond)

	return rmq
}

func (r *RMQ) BuildTopic(exchange string, queue string, topicType TopicTypes) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s", types.API, types.API_VERSION, exchange, queue, topicType)
}

func (r *RMQ) Publish(topic string, message []byte) {
	r.log.LogInfof("Publish() Locked()...")

	r.mutx.pubMutex.Lock()
	topicData, exists := r.subscribers[topic]
	r.mutx.pubMutex.Unlock()

	if exists {
		correlationID := uuid.New().String()

		// CHANGE: Use a DOT (.) instead of a SLASH (/)
		// This ensures the routing key matches the "api.v1.etc.#" binding
		routingKey := fmt.Sprintf("%s.%s", topicData.topic, correlationID)

		publish := amqp.Publishing{
			MessageId:     uuid.New().String(),
			DeliveryMode:  amqp.Persistent,
			Timestamp:     time.Now().UTC(),
			CorrelationId: correlationID,
			ContentType:   types.APPLICATION_JSON,
			Body:          message,
			AppId:         r.appID,
			Expiration:    MSG_EXPIREY_TIME,
		}

		err := r.Channel.PublishWithContext(r.Ctx, topicData.exchangeName, routingKey, false, false, publish)
		utils.HandleError(err, "Publish()", "Error publishing message...", r.log)
		r.log.LogInfof("Published message to: %s", routingKey)
	}
}

func (r *RMQ) Subscribe(topic string) {
	r.mutx.subscribeMutex.Lock()
	defer r.mutx.subscribeMutex.Unlock()

	// Split by DOT now
	splitTopic := strings.Split(topic, ".")
	if len(splitTopic) < 5 {
		r.log.LogErrorf("Invalid topic format: %s", topic)
		return
	}

	// api.v1.users_ex.users_queue.request
	// [0] [1]   [2]        [3]       [4]
	exchange := splitTopic[2]
	queueName := splitTopic[3]
	topicType := splitTopic[4]

	if _, exists := r.subscribers[topic]; !exists {
		switch topicType {
		case "request":
			err := r.Channel.ExchangeDeclare(exchange, amqp.ExchangeTopic, false, false, false, false, nil)
			utils.HandleError(err, "Subscribe()", "Error creating exchange...", r.log)

			_, err = r.Channel.QueueDeclare(queueName, false, false, false, false, nil)
			utils.HandleError(err, "Subscribe()", "Error creating queue...", r.log)

			// BIND WITH DOT WILDCARD
			bindingKey := fmt.Sprintf("%s.#", topic)
			err = r.Channel.QueueBind(queueName, bindingKey, exchange, false, nil)
			utils.HandleError(err, "Subscribe()", "Error binding queue...", r.log)

			r.subscribers[topic] = subscriber{
				topic:            topic,
				exchangeName:     exchange,
				queueName:        queueName,
				subscriptionType: topicType,
				appId:            r.appID,
				isAck:            false,
				exchangeType:     amqp.ExchangeTopic,
			}
		case "event":
			r.log.LogWarnf("Event logic not implemented yet")
		}
	}
}

func (r *RMQ) PubSub(ctx context.Context, topic string, message []byte) (*RmqPubSubResponse, error) {
	r.mutx.pubSubMutex.Lock()
	topicData, exists := r.subscribers[topic]
	r.mutx.pubSubMutex.Unlock()

	if !exists {
		return nil, fmt.Errorf("topic %s does not exist", topic)
	}

	// 1. Create an exclusive, auto-delete, temporary queue for the response
	q, err := r.Channel.QueueDeclare(
		"",    // name (empty means server generates one)
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare reply queue: %w", err)
	}

	// 2. Start consuming from the reply queue
	msgs, err := r.Channel.Consume(
		q.Name,
		"",    // consumer tag
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to consume from reply queue: %w", err)
	}

	correlationID := uuid.New().String()
	routingKey := fmt.Sprintf("%s.%s", topicData.topic, correlationID)

	publish := amqp.Publishing{
		MessageId:     uuid.New().String(),
		DeliveryMode:  amqp.Transient, // RPC responses don't usually need to be persistent
		Timestamp:     time.Now().UTC(),
		CorrelationId: correlationID,
		ReplyTo:       q.Name,
		ContentType:   types.APPLICATION_JSON,
		Body:          message,
		AppId:         r.appID,
		Expiration:    MSG_EXPIREY_TIME,
	}

	// 3. Publish using the passed-in context routingKey is whats used for matchinh consumer/producwr
	err = r.Channel.PublishWithContext(ctx, topicData.exchangeName, routingKey, false, false, publish)
	if err != nil {
		return nil, fmt.Errorf("failed to publish: %w", err)
	}

	//

	// 4. Wait for response or context cancellation
	for {
		select {
		case d, ok := <-msgs:
			if !ok {
				return nil, errors.New("response channel closed")
			}
			if d.CorrelationId == correlationID {
				r.log.LogInfof("PubSub Success for ID: %s", correlationID)
				return &RmqPubSubResponse{Service: d.AppId, Payload: d.Body}, nil
			}
		case <-ctx.Done():
			// If the HTTP request times out, we exit immediately
			return nil, ctx.Err()
		}
	}
}

func (r *RMQ) Consume(topic string) {
	r.mutx.consumeMutex.Lock()
	topicData, exists := r.subscribers[topic]
	r.mutx.consumeMutex.Unlock()

	if !exists {
		r.log.LogErrorf("Consume() topic %s does not exist", topic)
		return
	}

	// 1. Setup the consumer
	msgs, err := r.Channel.Consume(
		topicData.queueName,
		fmt.Sprintf("%s-worker-%s", r.appID, uuid.New().String()[:8]), // Unique worker tag
		false, // Manual Ack: We take responsibility for the message
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		r.log.LogErrorf("Error creating consumer: %s", err)
		return
	}

	go func() {
		for d := range msgs {
			r.log.LogInfof("Received Request ID: %s", d.CorrelationId)

			// 2. Logic: If the Gateway asked for a reply, we MUST send one.
			if d.ReplyTo != "" {
				// In a real app, you'd pass d.Body to a handler function here.
				responseBody := []byte(`{"status": "success", "message": "Processed by worker"}`)

				err := r.Channel.PublishWithContext(
					r.Ctx,
					"",        // Default Exchange
					d.ReplyTo, // The private callback queue name
					false,
					false,
					amqp.Publishing{
						ContentType:   "application/json",
						CorrelationId: d.CorrelationId, // MUST match the original ID
						Body:          responseBody,
						Timestamp:     time.Now().UTC(),
						AppId:         r.appID,
					},
				)

				if err != nil {
					r.log.LogErrorf("Failed to send reply to %s: %v", d.ReplyTo, err)
					// If we can't reply, we might want to Nack so another worker can try
					d.Nack(false, true)
					continue
				}
			}

			// 3. Finalize: Tell RabbitMQ the message is done.
			d.Ack(false)
		}
	}()
}

func (r *RMQ) Listen(topic string, handler func(*models.MessagePayload) ([]byte, error)) {
	topicData, exists := r.subscribers[topic]
	if !exists {
		r.log.LogErrorf("Cannot listen to non-existent topic: %s", topic)
		return
	}

	msgs, err := r.Channel.Consume(
		topicData.queueName,
		fmt.Sprintf("%s-worker-%s", r.appID, uuid.New().String()[:8]),
		false, // Manual Ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		r.log.LogErrorf("RMQ::Listen() Error: %s", err)
		return
	}

	go func() {
		for {
			select {
			case <-r.Ctx.Done():
				return
			case d, ok := <-msgs:
				if !ok {
					return
				}

				var req models.MessagePayload
				if err := json.Unmarshal(d.Body, &req); err != nil {
					r.log.LogErrorf("Bad JSON: %s", err)
					d.Nack(false, false) // Drop bad message
					continue
				}

				// Execute logic
				responseBytes, err := handler(&req)

				if d.ReplyTo != "" {
					// Send response back to Gateway
					pubErr := r.Channel.PublishWithContext(r.Ctx,
						"",        // Default Exchange
						d.ReplyTo, // Direct to the callback queue
						false,
						false,
						amqp.Publishing{
							ContentType:   types.APPLICATION_JSON,
							CorrelationId: d.CorrelationId,
							Body:          responseBytes,
							Timestamp:     time.Now().UTC(),
							AppId:         r.appID,
						},
					)
					if pubErr != nil {
						r.log.LogWarnf("Could not reply (Gateway likely timed out): %v", pubErr)
					}
				}

				// If handler failed, Nack so another worker can try.
				// If succeeded, Ack.
				if err != nil {
					d.Nack(false, true)
				} else {
					d.Ack(false)
				}
			}
		}
	}()
}

func (r *RMQ) ClearSubscriptions() {
	r.mutx.subscribeMutex.Lock()
	defer r.mutx.subscribeMutex.Unlock()
	r.subscribers = make(map[string]subscriber)
}

func (r *RMQ) Close() error {
	r.ClearSubscriptions()
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.Connection != nil {
		return r.Connection.Close()
	}
	return nil
}

// func (r *RMQ) handlePublish(topic string, requestPayload *models.MessagePayload, response []byte) {
// 	switch requestPayload.Event {
// 	case events.GET_USERS:
// 		r.Publish(topic, response)
// 	default:
// 		r.log.LogWarnf("method not implemented...")
// 	}
// }
