package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/types"
)

type EnvironmentVariablesConfig struct {
	CurrentEnv              string `env:"CURRENT_ENV" envDefault:"dev"`
	MqttBrokerURL           string `env:"MQTT_BROKER_URL" envDefault:"127.0.0.1:1883"`
	MqttBrokerURLTLS        string `env:"MQTT_BROKER_URL_TLS" envDefault:""`
	MqttUser                string `env:"MQTT_USER" envDefault:"user"`
	MqttPassword            string `env:"MQTT_PASSWORD" envDefault:"password"`
	RedisURL                string `env:"REDIS_URL" envDefault:"127.0.0.1:6379"`
	RedisPassword           string `env:"REDIS_PASSWORD" envDefault:"password"`
	MongoHost               string `env:"MONGO_HOST" envDefault:"127.0.0.1:27017"`
	MongoInitDBRootUsername string `env:"MONGO_INITDB_ROOT_USERNAME" envDefault:"user"`
	MongoInitDBRootPassword string `env:"MONGO_INITDB_ROOT_PASSWORD" envDefault:"password"`
}

// EnvConfig is a global variable that holds
// the environment configuration for the
// application. It is initialized by calling
// NewEnvironmentVariablesConfig() during application startup.
var EnvConfig *EnvironmentVariablesConfig = nil

// TODO: Handle case of different CURRENT_ENV values.
func NewEnvironmentVariablesConfig() {
	sync.OnceFunc(func() {
		var config EnvironmentVariablesConfig
		err := env.Parse(&config)
		if err != nil {
			fmt.Printf("Failed to parse environment variables using defaults. Error: %v", err)
			EnvConfig = &EnvironmentVariablesConfig{
				CurrentEnv:              "dev",
				MqttBrokerURL:           "127.0.0.1:1883",
				MqttBrokerURLTLS:        "",
				MqttUser:                "user",
				MqttPassword:            "password",
				RedisURL:                "127.0.0:1:6379",
				RedisPassword:           "password",
				MongoHost:               "127.0.0.1:27017",
				MongoInitDBRootUsername: "user",
				MongoInitDBRootPassword: "password",
			}
		}
	})()
}

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

func GetBrokerConnection(clientID string, tls bool) map[string]string {
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
		types.MQTT_BROKER_URL: "127.0.0.1:1883",
		types.MQTT_USER:       "user",
		types.MQTT_PASSWORD:   "password",
	}

	// For PROXY TESTING
	// var brokerConnectionMap map[string]string = map[string]string{
	// 	types.MQTT_USER:     "user",
	// 	types.MQTT_PASSWORD: "password",
	// }
	// brokerConnectionMap[types.MQTT_BROKER_URL] = "127.0.0.1:1883"
	// return brokerConnectionMap
}

// func GetBrokerConnection(clientID string, isInternalBridge bool) map[string]string {
// 	var brokerConnectionMap map[string]string = map[string]string{
// 		types.MQTT_USER:     "user",
// 		types.MQTT_PASSWORD: "password",
// 	}

// 	// If Mochi itself is running this to connect to the backends:
// 	if isInternalBridge {
// 		switch clientID {
// 		case "client1":
// 			brokerConnectionMap[types.MQTT_BROKER_URL] = "127.0.0.1:1884" // Mosquitto 1
// 		case "client2":
// 			brokerConnectionMap[types.MQTT_BROKER_URL] = "127.0.0.1:1885" // Mosquitto 2
// 		}
// 		return brokerConnectionMap
// 	}

// 	// Edge clients calling this always point to Mochi's front door
// 	brokerConnectionMap[types.MQTT_BROKER_URL] = "127.0.0.1:1883"
// 	return brokerConnectionMap
// }

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
