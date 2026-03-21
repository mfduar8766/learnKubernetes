package rmq

import (
	"context"
	"encoding/json"
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
	utils.HandleError(err, "NewRmq()", "Error connecting...", log)

	channel, err := connection.Channel()
	utils.HandleError(err, "NewRmq()", "Error creating channel...", log)

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

func (r *RMQ) PubSub(topic string, message []byte) (*RmqPubSubResponse, error) {
	r.mutx.pubSubMutex.Lock()
	topicData, exists := r.subscribers[topic]
	r.mutx.pubSubMutex.Unlock()

	if !exists {
		return nil, fmt.Errorf("topic %s does not exist", topic)
	}

	// Create private callback queue
	q, err := r.Channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return nil, err
	}

	msgs, err := r.Channel.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New().String()

	// USE DOT SEPARATOR
	routingKey := fmt.Sprintf("%s.%s", topicData.topic, correlationID)

	publish := amqp.Publishing{
		MessageId:     uuid.New().String(),
		DeliveryMode:  amqp.Persistent,
		Timestamp:     time.Now().UTC(),
		CorrelationId: correlationID,
		ReplyTo:       q.Name,
		ContentType:   types.APPLICATION_JSON,
		Body:          message,
		AppId:         r.appID,
	}

	pubCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = r.Channel.PublishWithContext(pubCtx, topicData.exchangeName, routingKey, false, false, publish)
	if err != nil {
		return nil, err
	}

	responseChan := make(chan *RmqPubSubResponse, 1)
	go func() {
		for d := range msgs {
			if d.CorrelationId == correlationID {
				responseChan <- &RmqPubSubResponse{Service: d.AppId, Payload: d.Body}
				return
			}
		}
	}()

	select {
	case res := <-responseChan:
		r.log.LogInfof("PubSub Success for ID: %s", correlationID)
		return res, nil
	case <-time.After(2 * time.Second): // Increased to 2s for testing
		return nil, fmt.Errorf("request timed out: no response from service")
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
	if r.Channel == nil || r.Channel.IsClosed() {
		r.log.LogErrorf("RMQ::Listen()::failed - Channel is not open")
		return
	}

	topicData, exists := r.subscribers[topic]
	if !exists {
		r.log.LogErrorf("Cannot listen to non-existent topic: %s", topic)
		return
	}

	msgs, err := r.Channel.Consume(
		topicData.queueName,
		fmt.Sprintf("%s-worker-%s", r.appID, uuid.New().String()[:8]), // Unique tag
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		r.log.LogErrorf("RMQ::Listen()::received error: %s", err)
		return
	}

	go func() {
		for d := range msgs {
			fmt.Printf("DDDDDD: %+v\n", d)
			// 1. Unmarshal
			var req models.MessagePayload
			if err := json.Unmarshal(d.Body, &req); err != nil {
				r.log.LogErrorf("Bad message format: %s", err)
				d.Nack(false, false) // Requeue: false (drop bad JSON)
				continue
			}

			// 2. Run the Handler
			responseBytes, err := handler(&req)
			if err != nil {
				r.log.LogErrorf("Handler error: %s", err)
				// If DB is down, we might want to requeue: true
				d.Nack(false, true)
				continue // DO NOT RETURN; stay in the loop!
			}

			// 3. Send the response back
			if d.ReplyTo != "" {
				err := r.Channel.PublishWithContext(context.Background(),
					"",
					d.ReplyTo,
					false,
					false,
					amqp.Publishing{
						ContentType:   types.APPLICATION_JSON,
						CorrelationId: d.CorrelationId,
						Body:          responseBytes,
						AppId:         r.appID,
						Timestamp:     time.Now().UTC(),
						ReplyTo:       d.ReplyTo,
					},
				)
				if err != nil {
					r.log.LogErrorf("Failed to publish reply: %s", err)
				}
			}

			// 4. Finalize
			d.Ack(false)
		}
	}()
}

func (r *RMQ) ClearSubscriptions() {
	r.subscribers = nil
}

// func (r *RMQ) handlePublish(topic string, requestPayload *models.MessagePayload, response []byte) {
// 	switch requestPayload.Event {
// 	case events.GET_USERS:
// 		r.Publish(topic, response)
// 	default:
// 		r.log.LogWarnf("method not implemented...")
// 	}
// }
