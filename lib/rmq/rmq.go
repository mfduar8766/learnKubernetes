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
	"github.com/mfduar8766/learnKubernetes/lib/events"
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
	if value := os.Getenv("RABBITMQ_URI"); len(value) > 0 {
		rmqURL = value
	}
	log.LogInfof("RMQ::NewRmq():: connected to: %s", rmqURL)
	connection, err := amqp.Dial(rmqURL)
	utils.HandleError(err, "NewRmq()", "Error connecting to RabbitMq...", log)
	log.LogInfo(&logger.LoggerPayload{
		Message:  "Successfully connected to RabbitMQ instance",
		Value:    rmqURL,
		FileName: "rmq.go",
	})
	channel, err := connection.Channel()
	utils.HandleError(err, "NewRmq()", "Error creating channel connection...", log)
	if channel == nil {
		utils.HandleError(errors.New("channel does not exist"), "NewRmq()", "", log)
	}
	channel.Confirm(false)
	rmq := &RMQ{
		log:            log,
		Connection:     connection,
		Channel:        channel,
		Ctx:            ctx,
		appID:          fmt.Sprintf("%s:%s", appId, uuid.New().String()),
		terminatedChan: make(chan bool),
		subscribers:    map[string]subscriber{},
		mutx: Mutexs{
			pubSubMutex:    &sync.Mutex{},
			pubMutex:       &sync.Mutex{},
			consumeMutex:   &sync.Mutex{},
			subscribeMutex: &sync.Mutex{},
		},
	}
	go func() {
		sig := <-signalChannel
		switch sig {
		case os.Interrupt, syscall.SIGTERM, syscall.SIGINT:
			rmq.terminatedChan <- true
		}
	}()
	return rmq
}

func (r *RMQ) Consume(topic string, isPublish bool, response []byte) *models.MessagePayload {
	r.log.LogInfof("Consumer() Locked()...")
	r.mutx.consumeMutex.Lock()
	defer r.mutx.consumeMutex.Unlock()
	responseChan := make(chan *models.MessagePayload)
	if topicData, exists := r.subscribers[topic]; exists {
		consumerStr := r.BuildTopic(USERS_EX, USERS_QUEUE, Response)
		msg, err := r.Channel.Consume(topicData.queueName, consumerStr, false, false, false, false, nil)
		if err != nil {
			r.log.LogError(&logger.LoggerPayload{
				Message:  "Error creating consumer...",
				Value:    err.Error(),
				FileName: "rmq.go",
				Method:   "Consume()",
			})
			responseChan <- nil
			close(responseChan)
		}
		r.log.LogInfof("Consumer() Listening for incoming messages...")
		go func() {
			for msg := range msg {
				r.log.LogInfo(&logger.LoggerPayload{
					Message: "Received message from:",
					Value: map[string]interface{}{
						"MessegeID":     msg.MessageId,
						"Exchange":      msg.Exchange,
						"AppId":         msg.AppId,
						"Time":          msg.Timestamp.Local().UTC().String(),
						"DeliveryTag":   msg.DeliveryTag,
						"CorrelationId": msg.CorrelationId,
						"ConsumerTag":   msg.ConsumerTag,
						"ReplyTo":       msg.ReplyTo,
						"RoutingKey":    msg.RoutingKey,
					},
					FileName: "rmq.go",
					Method:   "Consume()",
				})
				splitRoutingKey := strings.Split(msg.RoutingKey, "/")
				if msg.CorrelationId != splitRoutingKey[len(splitRoutingKey)-1] {
					r.log.LogErrorf("CorrlationId and routingKeyId do not match exiting go routine")
					responseChan <- nil
					close(responseChan)
				}
				var requstPayload models.MessagePayload
				err := json.Unmarshal(msg.Body, &requstPayload)
				if err != nil {
					r.Channel.Nack(msg.DeliveryTag, false, false)
					r.log.LogErrorf("Could not unmarshall message and cannot ack message")
					responseChan <- nil
					close(responseChan)
				}
				err = r.Channel.Ack(msg.DeliveryTag, false)
				if err != nil {
					r.log.LogErrorf("Error ack message: %s", err.Error())
					responseChan <- nil
					close(responseChan)
				} else {
					responseChan <- &requstPayload
					close(responseChan)
				}
				if isPublish && response != nil {
					r.handlePublish(topic, &requstPayload, response)
				}
			}
		}()
	}

	select {
	case <-r.terminatedChan:
		r.log.LogWarnf("Consumer() received terminate cmd Unlocked()....")
		return nil
	case resData := <-responseChan:
		r.log.LogInfof("Consumer() Unlocked()...")
		return resData
	}
}

func (r *RMQ) Subscribe(topic string) {
	r.mutx.subscribeMutex.Lock()
	defer r.mutx.subscribeMutex.Unlock()
	splitTopic := strings.Split(topic, "/")
	queueName := splitTopic[3]
	exchange := splitTopic[2]
	topicType := splitTopic[4]
	if _, exists := r.subscribers[topic]; !exists {
		switch topicType {
		case "request":
			err := r.Channel.ExchangeDeclare(exchange, FANOUT, false, false, false, false, nil)
			utils.HandleError(err, "Subscribe()", "Error creating exchange...", r.log)
			_, err = r.Channel.QueueDeclare(queueName, false, false, false, false, nil)
			utils.HandleError(err, "Subscribe()", "Error creating queue...", r.log)
			err = r.Channel.QueueBind(queueName, topic, exchange, false, nil)
			utils.HandleError(err, "Subscribe()", "Error binding queue to exchange...", r.log)
			r.subscribers[topic] = subscriber{
				topic:            topic,
				exchangeName:     exchange,
				queueName:        queueName,
				subscriptionType: topicType,
				appId:            r.appID,
				isAck:            false,
				exchangeType:     FANOUT,
			}
		case "event":
			r.log.LogWarnf("Not implemented...")
			// TODO...
		}
	}
}

func (r *RMQ) BuildTopic(exchange string, queue string, topicType TopicTypes) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", types.API, types.API_VERSION, exchange, queue, topicType)
}

func (r *RMQ) Publish(topic string, message []byte) {
	r.log.LogInfof("Publish() Locked()...")
	r.mutx.pubMutex.Lock()
	defer r.mutx.pubMutex.Unlock()
	if topicData, exists := r.subscribers[topic]; exists {
		id := uuid.New().String()
		topicData.topic += fmt.Sprintf("/%s", id)
		publish := amqp.Publishing{
			MessageId:     uuid.New().String(),
			DeliveryMode:  amqp.Persistent,
			Timestamp:     time.Now().Local().UTC(),
			CorrelationId: id,
			ContentType:   types.APPLICATION_JSON,
			Body:          message,
			AppId:         r.appID,
		}
		err := r.Channel.PublishWithContext(r.Ctx, topicData.exchangeName, topicData.topic, false, false, publish)
		utils.HandleError(err, "Publish()", "Error publishing message...", r.log)
		r.log.LogInfof("Publishing message to topic: %s Unlocked()...", topicData.topic)
	}
}

func (r *RMQ) PubSub(topic string, message []byte) (*rmqPubSubResponse, error) {
	r.log.LogInfof("PubSub() Locked(....)")
	r.mutx.pubSubMutex.Lock()
	defer r.mutx.pubSubMutex.Unlock()
	if topicData, exists := r.subscribers[topic]; exists {
		id := uuid.New().String()
		topicData.topic += fmt.Sprintf("/%s", id)
		consumerStr := r.BuildTopic(topicData.exchangeName, topicData.queueName, Response)
		waitChan := make(chan bool)
		msg, err := r.Channel.Consume(topicData.queueName, consumerStr, true, false, false, false, nil)
		utils.HandleError(err, "NewPubSub()", "Error creating consumer...", r.log)
		publish := amqp.Publishing{
			MessageId:     uuid.New().String(),
			DeliveryMode:  amqp.Persistent,
			Timestamp:     time.Now().Local().UTC(),
			CorrelationId: id,
			ContentType:   types.APPLICATION_JSON,
			Body:          message,
			ReplyTo:       topicData.queueName,
			AppId:         r.appID,
		}
		r.log.LogInfof("PubSub() Publishing message to topic: %s...", topicData.topic)
		confirmation, err := r.Channel.PublishWithDeferredConfirmWithContext(r.Ctx, topicData.exchangeName, topicData.topic, false, false, publish)
		utils.HandleError(err, "NewPubsUB()", "Error publishing...", r.log)
		topicData.id = id

		// select {
		<-confirmation.Done()
		r.log.LogInfo(&logger.LoggerPayload{
			Message: "PubSub() Received confirmation from publisher",
			Value: map[string]interface{}{
				"Ack":      confirmation.Acked(),
				"Delivery": confirmation.DeliveryTag,
			},
		})
		// }
		var (
			response       models.MessagePayload
			pubSubResponse *rmqPubSubResponse = new(rmqPubSubResponse)
		)

		go func() {
			for m := range msg {
				r.log.LogInfo(&logger.LoggerPayload{
					Message: "PubSub() Received response:",
					Value: map[string]interface{}{
						"MessegeID":     m.MessageId,
						"Exchange":      m.Exchange,
						"AppId":         m.AppId,
						"Time":          m.Timestamp.Local().UTC().String(),
						"DeliveryTag":   m.DeliveryTag,
						"CorrelationId": m.CorrelationId,
						"ConsumerTag":   m.ConsumerTag,
						"ReplyTo":       m.ReplyTo,
						"RoutingKey":    m.RoutingKey,
					},
				})
				pubSubResponse.Service = m.AppId
				splitRoutingKey := strings.Split(m.RoutingKey, "/")
				if m.CorrelationId != splitRoutingKey[len(splitRoutingKey)-1] {
					r.log.LogErrorf("PubSub() CorrlationId and routingKeyId do not match exiting go routine")
					waitChan <- false
					close(waitChan)
				}
				err := json.Unmarshal(m.Body, &response)
				if err != nil {
					r.log.LogErrorf("error marshalling json: %s", err.Error())
					waitChan <- false
					close(waitChan)
				} else {
					delete(r.subscribers, topicData.topic)
					pubSubResponse.Payload = m.Body
					waitChan <- true
					close(waitChan)
				}
			}
		}()

		// select {
		value := <-waitChan
		if value {
			r.log.LogInfof("PubSub() Unlocked()...")
			return pubSubResponse, nil
		} else {
			r.log.LogInfof("PubSub() Unlocked()...")
			return nil, fmt.Errorf("cannot publish to topic: %s as it does not exist", topic)
		}
		// }
		// return pubSubResponse, nil
	}
	r.log.LogInfof("PubSub() Unlocked()...")
	return nil, fmt.Errorf("cannot publish to topic: %s as it does not exist", topic)
}

func (r *RMQ) ClearSubscriptions() {
	r.subscribers = nil
}

func (r *RMQ) handlePublish(topic string, requestPayload *models.MessagePayload, response []byte) {
	switch requestPayload.Event {
	case events.GET_USERS:
		r.Publish(topic, response)
	default:
		r.log.LogWarnf("method not implemented...")
	}
}

func (r *RMQ) handleConsumerTopic(topic string) {
	r.log.LogWarnf("Not impemented...")
	// TODO: Map consumer topic to proper queue and exchange
}
