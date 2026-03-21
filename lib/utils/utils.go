package utils

import (
	"encoding/json"
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
)

func HandleError(err error, method string, message string, log logger.ILogger) {
	if err != nil {
		log.LogError(&logger.LoggerPayload{
			Message: message,
			Value:   err.Error(),
			Method:  method,
		})
	}
}

func MustHandleError(method, message string, err error, log logger.ILogger) {
	if err != nil {
		log.LogError(&logger.LoggerPayload{
			Message: message,
			Value:   err.Error(),
			Method:  method,
		})
		panic(err)
	}
}

func JsonMarshall(payload any) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return payloadBytes, nil
}

func JsonUnMarshall(data []byte, payload any) error {
	if err := json.Unmarshal(data, payload); err != nil {
		return err
	}
	return nil
}

func GetDate() string {
	return time.Now().UTC().Format("2006-01-02:15:04:05.000")
}

func BuildHttpError(err error, message, agent, host string) map[string]any {
	return map[string]any{
		"error":   err.Error(),
		"message": message,
		"time":    GetDate(),
		"agent":   agent,
		"url":     host,
	}
}
