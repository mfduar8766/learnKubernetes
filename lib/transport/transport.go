package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/paho"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	protos "github.com/mfduar8766/learnKubernetes/lib/protos/generated"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"google.golang.org/protobuf/proto"
)

type Transport struct {
	client          *paho.Client
	logger          logger.ILogger
	router          *paho.StandardRouter
	pendingRequests map[string]string
	// ProtoBuf MUST be a * because the internal code has Mutexes.
	requestResponseChan map[string]chan *protos.BrokerResponse
	reqResLock          sync.RWMutex
	topicAliasLock      sync.RWMutex
	topicAliasMap       map[string]string
}

func New(log logger.ILogger) *Transport {
	return &Transport{
		logger:              log,
		router:              paho.NewStandardRouter(),
		pendingRequests:     make(map[string]string),
		requestResponseChan: make(map[string]chan *protos.BrokerResponse),
		topicAliasMap:       make(map[string]string),
	}
}

func (b *Transport) Connect(ctx context.Context, clientID string, tls bool) error {
	brokerConnection := utils.GetBrokerConnection(tls)
	b.logger.LogInfo(&logger.LoggerPayload{Message: "Connecting to broker", Value: map[string]string{
		"brokerURL": brokerConnection[types.MQTT_BROKER_URL],
		"user":      brokerConnection[types.MQTT_USER],
		"password":  brokerConnection[types.MQTT_PASSWORD],
	}})

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", brokerConnection[types.MQTT_BROKER_URL])
	if err != nil {
		b.logger.LogErrorf("Failed to connect to broker at %s: %v", brokerConnection[types.MQTT_BROKER_URL], err)
		return fmt.Errorf("network dial failed: %w", err)
	}

	b.client = paho.NewClient(paho.ClientConfig{
		Conn: conn,
		OnPublishReceived: []func(paho.PublishReceived) (bool, error){
			b.handleIncomingPublish,
		},
		OnClientError:      b.onError,
		OnServerDisconnect: b.onServerDisconnect,
	})

	willPayload, err := utils.JsonMarshall(ServiceStatus{
		Event:  SERVICE_STATUS,
		Status: OFFLINE,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal LWT: %w", err)
	}

	connack, err := b.client.Connect(ctx, &paho.Connect{
		KeepAlive:    KEEP_ALIVE,
		CleanStart:   true,
		Username:     brokerConnection[types.MQTT_USER],
		Password:     []byte(brokerConnection[types.MQTT_PASSWORD]),
		ClientID:     clientID,
		UsernameFlag: true,
		PasswordFlag: true,
		Properties: &paho.ConnectProperties{
			SessionExpiryInterval: SESSION_EXPIRY,
		},
		WillProperties: &paho.WillProperties{
			PayloadFormat: paho.Byte(PAYLOAD_FORMAT_UTF_8),
			ContentType:   types.HEADER_APPLICATION_JSON,
		},
		WillMessage: &paho.WillMessage{
			Topic:   fmt.Sprintf("%s/%s/status", API_VERSION, clientID),
			Payload: willPayload,
			QoS:     DEFAULT_QoS,
			Retain:  true,
		},
	})

	if err != nil {
		return fmt.Errorf("mqtt connect failed: %w", err)
	}
	if connack.ReasonCode != 0 {
		return fmt.Errorf("broker rejected connection (Reason Code: %d)", connack.ReasonCode)
	}

	b.logger.LogInfo(&logger.LoggerPayload{Message: "Connected to broker", Value: brokerConnection[types.MQTT_BROKER_URL]})

	willPayload, err = utils.JsonMarshall(ServiceStatus{
		Event:  SERVICE_STATUS,
		Status: ONLINE,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal LWT: %w", err)
	}
	b.Publish(ctx, fmt.Sprintf("%s/%s/status", API_VERSION, clientID), willPayload, &PublishRequest{
		QoS:    DEFAULT_QoS,
		Retain: true,
	})
	return nil
}

func (b *Transport) Close() error {
	return b.client.Disconnect(&paho.Disconnect{})
}

func (b *Transport) BuildTopic(topicType TOPIC_TYPE, domains ...string) string {
	if topicType == TOPIC_TYPE_EVENT {
		return fmt.Sprintf("%s/%s/%s", API_VERSION, strings.Join(domains, TOPIC_SEPERATOR), topicEvent)
	} else {
		return fmt.Sprintf("%s/%s/%s", API_VERSION, strings.Join(domains, TOPIC_SEPERATOR), topicRequest)
	}
}

func (b *Transport) GetTopicType(topic string) *TopicProperties {
	topicParts := strings.Split(topic, TOPIC_SEPERATOR)
	if len(topicParts) < 3 {
		return nil
	}
	topicType := topicParts[len(topicParts)-1]
	domains := topicParts[1 : len(topicParts)-1]
	switch topicType {
	case topicEvent:
		return &TopicProperties{
			TopicType: TOPIC_TYPE_EVENT,
			Domains:   domains,
			Topic:     topic,
		}
	case topicRequest:
		return &TopicProperties{
			TopicType: TOPIC_TYPE_REQUEST,
			Domains:   domains,
			Topic:     topic,
		}
	default:
		return nil
	}
}

func (b *Transport) RegisterHandler(topic string, h paho.MessageHandler) {
	b.router.RegisterHandler(topic, h)
}

func (b *Transport) UnregisterHandler(topic string) {
	b.router.UnregisterHandler(topic)
}

func (b *Transport) Publish(ctx context.Context, topic string, payload []byte, properties *PublishRequest) (string, error) {
	return b.publish(ctx, topic, payload, properties)
}

func (b *Transport) PublishWithResponse(ctx context.Context, topic string, payload []byte, properties *PublishRequest) <-chan *protos.BrokerResponse {
	var responseChan chan *protos.BrokerResponse = make(chan *protos.BrokerResponse, 1)
	var wg sync.WaitGroup

	if properties == nil {
		responseID := utils.NewUUID()
		properties = &PublishRequest{
			Topic: topic,
			QoS:   DEFAULT_QoS,
			Properties: &PublishProperties{
				ContentType:     types.HEADER_APPLICATION_PROTO,
				PayloadFormat:   DEFAULT_PAYLOAD_FORMAT,
				MessageExpiry:   MESSAGE_EXPIRY,
				CorrelationData: []byte(responseID),
			},
		}
	}
	if properties != nil && properties.Properties == nil {
		responseID := utils.NewUUID()
		properties.Properties = &PublishProperties{
			ContentType:     types.HEADER_APPLICATION_PROTO,
			PayloadFormat:   DEFAULT_PAYLOAD_FORMAT,
			MessageExpiry:   MESSAGE_EXPIRY,
			CorrelationData: []byte(responseID),
		}
	}

	corID := string(properties.Properties.CorrelationData)
	waitCh := make(chan *protos.BrokerResponse, 1)
	b.reqResLock.Lock()
	b.requestResponseChan[corID] = waitCh
	b.reqResLock.Unlock()

	wg.Go(func() {
		defer func() {
			b.reqResLock.Lock()
			delete(b.requestResponseChan, corID)
			b.reqResLock.Unlock()
			close(responseChan)
		}()

		_, err := b.publish(ctx, topic, payload, properties)
		if err != nil {
			responseChan <- &protos.BrokerResponse{Error: err.Error()} //protos.BrokerResponse{Error: err}
			return
		}

		select {
		case <-ctx.Done():
			responseChan <- &protos.BrokerResponse{TimeOut: true, Error: ctx.Err().Error()}
		case res, ok := <-waitCh:
			if ok {
				responseChan <- res
			}
		}
	})
	wg.Wait()
	return responseChan
}

func (t *Transport) SubscribeMultiple(topics ...string) {
	for _, topic := range topics {
		t.Subscribe(context.Background(), topic, nil)
	}
}

func (t *Transport) UnsubscribeMultiple(topics ...string) {
	t.Unsubscribe(context.Background(), topics...)
}

func (b *Transport) Subscribe(ctx context.Context, topic string, properties *SubscribeProperties) error {
	subscribeProperties := paho.Subscribe{
		Properties: &paho.SubscribeProperties{},
		Subscriptions: []paho.SubscribeOptions{
			{
				Topic: topic,
				QoS:   DEFAULT_QoS,
			},
		},
	}
	if properties != nil {
		//
	}
	_, err := b.client.Subscribe(ctx, &subscribeProperties)
	if err != nil {
		return err
	}
	return nil
}

func (b *Transport) Unsubscribe(ctx context.Context, topics ...string) []string {
	var failedTopics []string

	if len(topics) == 0 {
		return nil
	}

	unsubAck, err := b.client.Unsubscribe(ctx, &paho.Unsubscribe{
		Topics: topics,
	})

	if err != nil {
		b.logger.LogErrorf("Network error during unsubscribe: %v", err)
		return topics
	}

	// Inspect individual results (MQTT v5 feature)
	// The UnsubAck ReasonCodes match the order of the topics sent
	for i, code := range unsubAck.Reasons {
		topic := topics[i]

		// ReasonCode 0x00 is Success. Anything else (like 0x11, 0x80) is a failure.
		if code > 0 {
			b.logger.LogWarnf("Transport rejected unsubscribe for %s: Code %v", topic, code)
			failedTopics = append(failedTopics, topic)
		} else {
			// Success! Clean up the local router memory
			b.router.UnregisterHandler(topic)
		}
	}

	return failedTopics
}

func CheckTransportResponseForErrors[T proto.Message](
	w http.ResponseWriter,
	r *http.Request,
	request *models.MessagePayload[T],
	response *protos.BrokerResponse,
	ok bool,
) error {
	var err error
	var statusCode int
	var displayError string

	if !ok {
		statusCode = http.StatusInternalServerError
		displayError = "Internal server error"
		goto SEND_ERROR
	}

	if len(response.Error) > 0 {
		statusCode = http.StatusInternalServerError
		displayError = fmt.Sprintf("Service error: %v", response.Error)
		err = errors.New(response.Error)
		goto SEND_ERROR
	}

	if response.TimeOut {
		statusCode = http.StatusGatewayTimeout
		displayError = "Request timed out"
		goto SEND_ERROR
	}

	if len(response.Payload) == 0 {
		statusCode = http.StatusNoContent
		displayError = "No data found"
		goto SEND_ERROR
	}

	return nil

SEND_ERROR:
	w.WriteHeader(statusCode)
	request.Error = utils.BuildHttpError(err, displayError, r.UserAgent(), r.Host)
	errorBytes, err := request.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.Write(errorBytes)
	return err
}

func CreatePayloadFormat(contentType uint8) *byte {
	switch contentType {
	case PAYLOAD_FORMAT_BYTES:
		return paho.Byte(PAYLOAD_FORMAT_BYTES)
	case PAYLOAD_FORMAT_UTF_8:
		return paho.Byte(PAYLOAD_FORMAT_UTF_8)
	default:
		return DEFAULT_PAYLOAD_FORMAT
	}
}

func (b *Transport) publish(ctx context.Context, topic string, payload []byte, properties *PublishRequest) (string, error) {
	topicType := b.GetTopicType(topic)
	subQoS := DEFAULT_QoS
	publish := &paho.Publish{
		Topic:   topic,
		Payload: payload,
		QoS:     DEFAULT_QoS,
		Retain:  false,
		Properties: &paho.PublishProperties{
			ContentType:   types.HEADER_APPLICATION_PROTO,
			PayloadFormat: DEFAULT_PAYLOAD_FORMAT,
			MessageExpiry: MESSAGE_EXPIRY,
		},
	}
	if strings.Contains(topic, "request/") {
		publish.Properties.ResponseTopic = topic
		splitTopic := strings.Split(topic, TOPIC_SEPERATOR)
		responseID := splitTopic[len(splitTopic)-1]
		publish.Properties.CorrelationData = []byte(responseID)
		publish.Properties.User.Add(from, topic)
	}
	if properties != nil {
		publish.Retain = properties.Retain
		publish.QoS = properties.QoS
		if properties.Properties != nil {
			subQoS = int(properties.QoS)
			publish.Properties.PayloadFormat = properties.Properties.PayloadFormat
			publish.Properties.MessageExpiry = properties.Properties.MessageExpiry
			publish.Properties.TopicAlias = properties.Properties.TopicAlias
			publish.Properties.User = properties.Properties.User
		}
	}
	if topicType != nil && topicType.TopicType == TOPIC_TYPE_REQUEST {
		var responseID string
		if properties != nil && properties.Properties != nil {
			if len(properties.Properties.CorrelationData) > 0 {
				responseID = string(properties.Properties.CorrelationData)
			} else {
				responseID = utils.NewUUID()
			}
		}
		responseTopic := fmt.Sprintf("%s/%s", topic, responseID)
		publish.Properties.ResponseTopic = responseTopic
		publish.Properties.CorrelationData = []byte(responseID)
		publish.Properties.User.Add(to, topic)

		b.reqResLock.Lock()
		b.pendingRequests[responseID] = responseID
		b.reqResLock.Unlock()

		err := b.Subscribe(ctx, responseTopic, &SubscribeProperties{
			QoS: byte(subQoS),
		})
		if err != nil {
			return "", err
		}
		_, err = b.client.Publish(ctx, publish)
		if err != nil {
			return "", err
		}
		return responseID, nil
	}
	_, err := b.client.Publish(ctx, publish)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (b *Transport) handleIncomingPublish(pr paho.PublishReceived) (bool, error) {
	if pr.Packet == nil {
		return true, nil
	}

	// b.logger.LogDebug(&logger.LoggerPayload{
	// 	Message: "handleIncomingPublish()::Msg",
	// 	Value: map[string]any{
	// 		"Topic":          pr.Packet.Topic,
	// 		"ResponseTopic":  pr.Packet.Properties.ResponseTopic,
	// 		"CorID":          string(pr.Packet.Properties.CorrelationData),
	// 		"Payload":        string(pr.Packet.Payload),
	// 		"userProperties": pr.Packet.Properties.User,
	// 	},
	// })

	// Check for correlation data for Request-Response pattern
	if len(pr.Packet.Properties.CorrelationData) > 0 {
		correlationID := string(pr.Packet.Properties.CorrelationData)
		b.reqResLock.Lock()
		responseTopic := pr.Packet.Properties.User.Get(from)

		_, exists := b.pendingRequests[correlationID]
		if exists && len(from) > 0 && len(pr.Packet.Properties.ResponseTopic) > 0 && pr.Packet.Properties.ResponseTopic == responseTopic {
			b.requestResponseChan[correlationID] <- &protos.BrokerResponse{
				Payload:       pr.Packet.Payload,
				Topic:         pr.Packet.Topic,
				CorrelationId: correlationID,
				ResponseTopic: pr.Packet.Properties.ResponseTopic,
			}
			close(b.requestResponseChan[correlationID])
			delete(b.pendingRequests, correlationID)
			b.reqResLock.Unlock()
			return true, nil
		}
		b.reqResLock.Unlock()
	}

	// Route to standard handlers if no correlation match
	b.router.Route(pr.Packet.Packet())
	return true, nil
}

func (b *Transport) onError(err error) {
	b.logger.LogErrorf("MQTT client error: %v", err)
}

func (b *Transport) onServerDisconnect(d *paho.Disconnect) {
	b.logger.LogWarnf("Server disconnected. Reason: %d", d.ReasonCode)
}
