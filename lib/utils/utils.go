package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/types"
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
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	return map[string]any{
		"error":   errorMessage,
		"message": message,
		"time":    GetDate(),
		"agent":   agent,
		"url":     host,
	}
}

func GetCurrentENV() string {
	return os.Getenv(types.CURRENT_ENV)
}

func GetHostPort(serviceName string) int {
	currentEnv := os.Getenv(types.CURRENT_ENV)
	if currentEnv == types.PROD_ENV {
		if port := os.Getenv(types.HOST_PORT); len(port) > 0 {
			intPort, err := strconv.Atoi(port)
			if err != nil {
				panic(err)
			}
			return intPort
		}
	}
	servicesMap := map[string]int{
		types.APP_GATE_WAY:      3000,
		types.APP_USERS_SERVICE: 3001,
	}
	hostPort, exist := servicesMap[serviceName]
	if !exist {
		panic(fmt.Sprintf("service: %s does not exist", serviceName))
	}
	return hostPort
}

func NewUUID() string {
	return uuid.New().String()
}

func GetEnv(env string) string {
	return strings.TrimSpace(os.Getenv(env))
}

func GetBrokerConnection(tls bool) map[string]string {
	currentENV := GetCurrentENV()
	if currentENV == types.PROD_ENV {
		brokerURL := GetEnv(types.MQTT_BROKER_URL)
		if tls {
			brokerURL = GetEnv(types.MQTT_BROKER_URL_TLS)
		}

		brokerUser := GetEnv(types.MQTT_USER)
		brokerPassword := GetEnv(types.MQTT_PASSWORD)
		return map[string]string{
			types.MQTT_BROKER_URL: brokerURL,
			types.MQTT_USER:       brokerUser,
			types.MQTT_PASSWORD:   brokerPassword,
		}

	}
	return map[string]string{
		types.MQTT_BROKER_URL: "localhost:1883",
		types.MQTT_USER:       "user",
		types.MQTT_PASSWORD:   "password",
	}
}
