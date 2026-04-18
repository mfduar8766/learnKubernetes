package models

import (
	"encoding/json"

	"github.com/mfduar8766/learnKubernetes/lib/events"
	"github.com/mfduar8766/learnKubernetes/lib/types"
)

type Posts struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type ResponsePayloadParams[T any] struct {
	Result types.ResponsePayloadResults `json:"result"`
	Data   T                            `json:"data"`
}

type MessagePayload[T any] struct {
	Event    events.UserEventsType `json:"event"`
	*Params  `json:"params,omitempty"`
	Response *ResponsePayloadParams[T] `json:"response,omitempty"`
	Error    map[string]any            `json:"error,omitempty"`
	Topic    string                    `json:"topic"`
}

func CreateNewMessagePayload[T any](topic string, event events.UserEvents, params *Params, response *ResponsePayloadParams[T], errorMessage map[string]interface{}) *MessagePayload[T] {
	return &MessagePayload[T]{
		Event:    events.UserEventsType(event),
		Params:   params,
		Response: response,
		Error:    errorMessage,
		Topic:    topic,
	}
}

func (m *MessagePayload[T]) Marshall() ([]byte, error) {
	messageBytes, err := json.Marshal(m)
	return messageBytes, err
}

type Request[T any] struct {
	MetaData map[string]any `json:"metaData"`
	Body     T              `json:"body"`
}

func (r *Request[T]) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Request[T]) UnMarshal(data []byte) error {
	return json.Unmarshal(data, r)
}
