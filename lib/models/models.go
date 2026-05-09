package models

import (
	"encoding/json"

	protos "github.com/mfduar8766/learnKubernetes/lib/protos/generated"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type ResponsePayloadParams[T proto.Message] struct {
	Result protos.ResponsePayloadResults
	Data   T
}

type MessagePayload[T proto.Message] struct {
	Event         string
	Params        *protos.Params
	Response      *ResponsePayloadParams[T]
	Error         map[string]any
	ResponseTopic string
}

func CreateNewMessagePayload[T proto.Message](
	responseTopic string,
	event string,
	params *protos.Params,
	response *ResponsePayloadParams[T],
	errorMessage map[string]interface{},
) *MessagePayload[T] {
	return &MessagePayload[T]{
		Event:         event,
		Params:        params,
		Response:      response,
		Error:         errorMessage,
		ResponseTopic: responseTopic,
	}
}

func (m *MessagePayload[T]) ToProto() (*protos.MessagePayload, error) {
	var anyData *anypb.Any = nil
	var err error
	var response *protos.ResponsePayload = nil

	// 1. Pack the generic Data into the Any field
	if m.Response != nil && any(m.Response.Data) != nil {
		anyData, err = anypb.New(m.Response.Data)
		if err != nil {
			return nil, err
		}
		response = &protos.ResponsePayload{
			Result: m.Response.Result,
			Data:   anyData,
		}
	}

	// 2. Convert Error map to Struct
	errStruct, err := structpb.NewStruct(m.Error)
	if err != nil {
		return nil, err
	}

	// 3. Map everything to the generated Proto struct
	return &protos.MessagePayload{
		Event:         m.Event,
		Params:        m.Params,
		ResponseTopic: m.ResponseTopic,
		Response:      response,
		Error:         errStruct,
	}, nil
}

func (m *MessagePayload[T]) Marshal() ([]byte, error) {
	protoMsg, err := m.ToProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(protoMsg)
}

type LogLevelRequest struct {
	LogLevel string `json:"level" required:"true"`
}

func (l *LogLevelRequest) Marshal() ([]byte, error) {
	return json.Marshal(l)
}

func (l *LogLevelRequest) UnMarshal(data []byte) error {
	return json.Unmarshal(data, l)
}

type LogLevelResponse struct {
	Result   types.ResponsePayloadResults `json:"result"`
	LogLevel string                       `json:"level" required:"true"`
}

func (l *LogLevelResponse) Marshal() ([]byte, error) {
	return json.Marshal(l)
}

func (l *LogLevelResponse) UnMarshal(data []byte) error {
	return json.Unmarshal(data, l)
}
