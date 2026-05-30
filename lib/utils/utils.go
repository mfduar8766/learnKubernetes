package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	return GetEnv(types.CURRENT_ENV)
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

func CreateDbConnectionString(dbType types.DbType, log logger.ILogger) (string, error) {
	var (
		currentENV       = GetCurrentENV()
		connectionString = ""
	)
	switch dbType {
	case types.DB_REDIS:
		if currentENV == types.PROD_ENV {
			var (
				redisHost     = "redis-service:6379"
				redisPassword = "password"
				host          = GetEnv(types.REDIS_URL)
				password      = GetEnv(types.REDIS_PASSWORD)
			)
			if len(host) == 0 || len(password) == 0 {
				log.LogWarnf("Utils::CreateDbConnectionString()::No redis host or password configured. Using default values instead")
			}
			connectionString = fmt.Sprintf("%s_%s", redisHost, redisPassword)
		} else {
			connectionString = fmt.Sprintf("%s:%d_%s", types.LOCAL_HOST, 6379, "password")
		}
	case types.DB_MONGO:
		if currentENV == types.PROD_ENV {
			k8sHost := "mongo-service:27017"
			user := GetEnv(types.MONGO_INITDB_ROOT_USERNAME)
			pass := GetEnv(types.MONGO_INITDB_ROOT_PASSWORD)
			host := GetEnv(types.MONGO_HOST)

			if host == "" {
				host = k8sHost
			}
			if user == "" {
				user = "user"
			}
			if pass == "" {
				pass = "password"
			}

			connectionString = fmt.Sprintf("mongodb://%s:%s@%s", user, pass, host)
		} else {
			connectionString = "mongodb://user:password@127.0.0.1:27017/?authSource=admin"
		}
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbType.String())
	}
	return connectionString, nil
}

// TODO: Update this to actually read the request body
func ReadRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}
