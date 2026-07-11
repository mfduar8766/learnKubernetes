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
	"github.com/mochi-mqtt/server/v2/packets"
	mPackets "github.com/mochi-mqtt/server/v2/packets"
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
	isConnected         bool
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
	brokerConnection := utils.GetBrokerConnection(clientID, tls)
	b.logger.LogInfo(&logger.LoggerPayload{Message: "Connecting to broker", Value: map[string]string{
		"brokerURL": brokerConnection[types.MQTT_BROKER_URL],
		"user":      brokerConnection[types.MQTT_USER],
		"password":  brokerConnection[types.MQTT_PASSWORD],
	}})

	var (
		defaultConnectTimeoutMS = 10000
		maxConnectRetries       = 5
		retryCount              = 0
		conn                    net.Conn
		connectError            error
	)

	for {
		if retryCount >= maxConnectRetries {
			return connectError
		}
		dialer := net.Dialer{Timeout: 10 * time.Second}
		conn, connectError = dialer.DialContext(ctx, "tcp", brokerConnection[types.MQTT_BROKER_URL])
		if connectError != nil {
			b.logger.LogErrorf("Failed to connect to broker at %s: %v", brokerConnection[types.MQTT_BROKER_URL], connectError)
			connectError = fmt.Errorf("network dial failed: %w", connectError)
			retryCount++
			select {
			case <-time.After(time.Duration(defaultConnectTimeoutMS) * time.Microsecond):
			case <-ctx.Done():
				return fmt.Errorf("context canceled while trying to connect: %w", ctx.Err())
			}
			continue
		}
		break
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

	// protocolErr := packets.ErrProtocolViolation
	// fmt.Printf("PROTO: %+v\n", protocolErr)

	connack, err := b.client.Connect(ctx, &paho.Connect{
		KeepAlive:    KEEP_ALIVE,
		CleanStart:   true,
		Username:     brokerConnection[types.MQTT_USER],
		Password:     []byte(brokerConnection[types.MQTT_PASSWORD]),
		ClientID:     clientID,
		UsernameFlag: true,
		PasswordFlag: true,
		Properties: &paho.ConnectProperties{
			SessionExpiryInterval: &SESSION_EXPIRY,
			RequestResponseInfo:   true,
			// THIS IS CAUSING THE BROKER TO NOT CONNECT. THIS IS BC CLEAN_START IS TRUE,
			// MEANING THE CLIENT IS NOT ALLOWING THE BROKER TO STORE THE LWT FOR WHEN IT DISCONNECTS.
			// THIS SHOULD BE FINE FOR OUR USE CASE BUT IF YOU WANT TO USE LWT WITH CLEAN_START TRUE,
			// YOU NEED TO SET A WILL_DELAY_INTERVAL IN THE CONNECT PROPERTIES AND SET IT TO A VALUE
			//  LOWER THAN THE SESSION_EXPIRY_INTERVAL CONFIGURED IN THE BROKER (mosquitto.conf) SO
			// THAT THE BROKER KNOWS TO WAIT THAT AMOUNT OF TIME BEFORE PUBLISHING THE LWT ON DISCONNECTION.
			// THIS WAY, IF THE CLIENT RECONNECTS WITHIN THAT TIME FRAME, THE BROKER WILL NOT PUBLISH THE
			// LWT AND WILL DELETE IT INSTEAD.
			// WillDelayInterval:     &WILL_DELAY_INTERVAL,
			RequestProblemInfo: true,
			TopicAliasMaximum:  &MAX_TOPIC_ALIAS,
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

	fmt.Printf("CONNACK_CODE: %+v\n", connack.ReasonCode)
	fmt.Printf("ERR: %+v\n", mPackets.ErrServerMoved)

	// --- REDIRECTION HANDLING ENGINE ---
	if connack.ReasonCode == mPackets.ErrServerMoved.Code || connack.ReasonCode == mPackets.ErrServerBusy.Code {
		if connack.Properties != nil && connack.Properties.ServerReference != "" {
			newServer := connack.Properties.ServerReference
			b.logger.LogInfo(&logger.LoggerPayload{
				Message: "Server redirected client",
				Value:   map[string]string{"RedirectedTo": newServer},
			})

			// Clean up the current network socket
			conn.Close()
		}
		conn.Close()
		return fmt.Errorf("server sent redirection code %x but omitted ServerReference", connack.ReasonCode)
	}

	if connack.ReasonCode != 0 {
		return fmt.Errorf("broker rejected connection (Reason Code: %d)", connack.ReasonCode)
	}

	b.isConnected = true
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

// func (b *Transport) Connect(ctx context.Context, clientID string, tls bool) error {
// 	brokerConnection := utils.GetBrokerConnection(clientID, tls)

// 	// Start with whatever URL your utility gives you
// 	targetURL := brokerConnection[types.MQTT_BROKER_URL]

// 	// Allow a few redirection hops
// 	for attempts := 0; attempts < 3; attempts++ {
// 		b.logger.LogInfo(&logger.LoggerPayload{Message: "Connecting to broker", Value: map[string]string{
// 			"brokerURL": targetURL,
// 			"user":      brokerConnection[types.MQTT_USER],
// 		}})

// 		dialer := net.Dialer{Timeout: 10 * time.Second}
// 		conn, err := dialer.DialContext(ctx, "tcp", targetURL)
// 		if err != nil {
// 			return fmt.Errorf("network dial failed to %s: %w", targetURL, err)
// 		}

// 		b.client = paho.NewClient(paho.ClientConfig{
// 			Conn: conn,
// 			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
// 				b.handleIncomingPublish,
// 			},
// 			OnClientError:      b.onError,
// 			OnServerDisconnect: b.onServerDisconnect,
// 		})

// 		willPayload, err := utils.JsonMarshall(ServiceStatus{
// 			Event:  SERVICE_STATUS,
// 			Status: OFFLINE,
// 		})
// 		if err != nil {
// 			conn.Close()
// 			return fmt.Errorf("failed to marshal LWT: %w", err)
// 		}

// 		connack, err := b.client.Connect(ctx, &paho.Connect{
// 			KeepAlive:    KEEP_ALIVE,
// 			CleanStart:   true,
// 			Username:     brokerConnection[types.MQTT_USER],
// 			Password:     []byte(brokerConnection[types.MQTT_PASSWORD]),
// 			ClientID:     clientID,
// 			UsernameFlag: true,
// 			PasswordFlag: true,
// 			Properties: &paho.ConnectProperties{
// 				SessionExpiryInterval: &SESSION_EXPIRY,
// 				RequestResponseInfo:   true,
// 				// THIS IS CAUSING THE BROKER TO NOT CONNECT. THIS IS BC CLEAN_START IS TRUE,
// 				// MEANING THE CLIENT IS NOT ALLOWING THE BROKER TO STORE THE LWT FOR WHEN IT DISCONNECTS.
// 				// THIS SHOULD BE FINE FOR OUR USE CASE BUT IF YOU WANT TO USE LWT WITH CLEAN_START TRUE,
// 				// YOU NEED TO SET A WILL_DELAY_INTERVAL IN THE CONNECT PROPERTIES AND SET IT TO A VALUE
// 				//  LOWER THAN THE SESSION_EXPIRY_INTERVAL CONFIGURED IN THE BROKER (mosquitto.conf) SO
// 				// THAT THE BROKER KNOWS TO WAIT THAT AMOUNT OF TIME BEFORE PUBLISHING THE LWT ON DISCONNECTION.
// 				// THIS WAY, IF THE CLIENT RECONNECTS WITHIN THAT TIME FRAME, THE BROKER WILL NOT PUBLISH THE
// 				// LWT AND WILL DELETE IT INSTEAD.
// 				// WillDelayInterval:     &WILL_DELAY_INTERVAL,
// 				RequestProblemInfo: true,
// 				TopicAliasMaximum:  &MAX_TOPIC_ALIAS,
// 			},
// 			WillMessage: &paho.WillMessage{
// 				Topic:   fmt.Sprintf("%s/%s/status", API_VERSION, clientID),
// 				Payload: willPayload,
// 				QoS:     DEFAULT_QoS,
// 				Retain:  true,
// 			},
// 		})
// 		if err != nil {
// 			conn.Close()
// 			return fmt.Errorf("mqtt connect failed: %w", err)
// 		}

// 		// --- FIXED REDIRECTION HANDLING ENGINE ---
// 		if connack.ReasonCode == mPackets.ErrServerMoved.Code {
// 			if connack.Properties != nil && connack.Properties.ServerReference != "" {
// 				// Update targetURL with the reference from Mochi (e.g. "127.0.0.1:1885")
// 				targetURL = connack.Properties.ServerReference

// 				b.logger.LogInfo(&logger.LoggerPayload{
// 					Message: "Server redirected client, retrying target...",
// 					Value:   map[string]string{"RedirectedTo": targetURL},
// 				})

// 				conn.Close() // Drop Mochi proxy socket
// 				continue     // Loop back up and dial the redirected targetURL!
// 			}
// 			conn.Close()
// 			return fmt.Errorf("server sent redirection code %x but omitted ServerReference", connack.ReasonCode)
// 		}

// 		if connack.ReasonCode != 0 {
// 			conn.Close()
// 			return fmt.Errorf("broker rejected connection (Reason Code: %d)", connack.ReasonCode)
// 		}

// 		// If we reach here, connection is successful (ReasonCode == 0)
// 		b.isConnected = true
// 		b.logger.LogInfo(&logger.LoggerPayload{Message: "Connected to broker", Value: targetURL})

// 		// Send online status
// 		onlinePayload, _ := utils.JsonMarshall(ServiceStatus{Event: SERVICE_STATUS, Status: ONLINE})
// 		b.Publish(ctx, fmt.Sprintf("%s/%s/status", API_VERSION, clientID), onlinePayload, &PublishRequest{
// 			QoS:    DEFAULT_QoS,
// 			Retain: true,
// 		})
// 		return nil
// 	}

// 	return fmt.Errorf("failed to connect: max redirection cycles exceeded")
// }

func (b *Transport) Client() *paho.Client {
	return b.client
}

func (b *Transport) IsConnectted() bool {
	return b.client != nil && b.isConnected
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
				MessageExpiry:   &MESSAGE_EXPIRY,
				CorrelationData: []byte(responseID),
			},
		}
	}
	if properties != nil && properties.Properties == nil {
		responseID := utils.NewUUID()
		properties.Properties = &PublishProperties{
			ContentType:     types.HEADER_APPLICATION_PROTO,
			PayloadFormat:   DEFAULT_PAYLOAD_FORMAT,
			MessageExpiry:   &MESSAGE_EXPIRY,
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

// TODO: FIx this later
func (t *Transport) SubscribeMultiple(topics ...string) {
	for _, topic := range topics {
		t.Subscribe(context.Background(), []paho.SubscribeOptions{
			{
				Topic: topic,
			},
		})
	}
}

func (t *Transport) UnsubscribeMultiple(topics ...string) {
	t.Unsubscribe(context.Background(), topics...)
}

func (b *Transport) Subscribe(ctx context.Context, options []paho.SubscribeOptions) error {
	subAck, err := b.client.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: options,
	})
	if err != nil {
		return fmt.Errorf("network error during subscribe: %w", err)
	}

	if subAck == nil {
		return fmt.Errorf("Transport::Subscribe()::subAck is nil")
	}

	for index, reason := range subAck.Reasons {
		// In MQTT v5, any reason code >= 0x80 (128) is an error/failure status code.
		if reason >= packets.ErrUnspecifiedError.Code {
			return fmt.Errorf("broker rejected subscription for topic index %d with reason code: 0x%X (%d)",
				index, reason, reason)
		}
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
			MessageExpiry: &MESSAGE_EXPIRY,
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

		err := b.Subscribe(ctx, []paho.SubscribeOptions{
			{
				Topic: responseTopic,
				QoS:   byte(subQoS),
			},
		})
		if err != nil {
			return "", err
		}
		pubRes, err := b.client.Publish(ctx, publish)
		if err != nil {
			return "", err
		}

		if pubRes == nil {
			return "", fmt.Errorf("Transport::publish()::no pub response from broker...")
		}

		if pubRes.ReasonCode >= packets.ErrUnspecifiedError.Code {
			return "", fmt.Errorf("Transport::publish()::publish response not received for topic: %s", responseTopic)
		}

		return responseID, nil
	}

	pubRes, err := b.client.Publish(ctx, publish)
	if err != nil {
		return "", err
	}
	if pubRes == nil {
		return "", fmt.Errorf("Transport::publish()::no pub response from broker...")
	}

	if pubRes.ReasonCode >= packets.ErrUnspecifiedError.Code {
		return "", fmt.Errorf("Transport::publish()::publish response not received for topic: %s", topic)
	}

	return "", nil
}

func (b *Transport) handleIncomingPublish(pr paho.PublishReceived) (bool, error) {
	if pr.Packet == nil {
		return true, nil
	}

	// Check for correlation data for Request-Response pattern
	if len(pr.Packet.Properties.CorrelationData) > 0 {
		correlationID := string(pr.Packet.Properties.CorrelationData)
		b.reqResLock.Lock()
		responseTopic := pr.Packet.Properties.User.Get(from)

		_, exists := b.pendingRequests[correlationID]
		if exists && len(pr.Packet.Properties.ResponseTopic) > 0 && pr.Packet.Properties.ResponseTopic == responseTopic {
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
	b.isConnected = false
}
