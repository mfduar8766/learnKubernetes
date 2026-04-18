package types

import "fmt"

type ApplicationRolesType string
type ResponsePayloadResultsType string
type ApplicationRoles ApplicationRolesType
type ResponsePayloadResults ResponsePayloadResultsType

const (
	UNKNOWN    ApplicationRolesType = "UNKNOWN"
	USER       ApplicationRolesType = "USER"
	ADMIN      ApplicationRolesType = "ADMIN"
	SUPER_USER ApplicationRolesType = "SUPER_USER"
)

const (
	SUCCESS               ResponsePayloadResultsType = "success"
	INTERNAL_SERVER_ERROR ResponsePayloadResultsType = "internalServerError"
	NOT_FOUND             ResponsePayloadResultsType = "notFound"
	RESOURCE_IN_USE       ResponsePayloadResultsType = "resourceInUse"
)

const (
	API                         = "api"
	API_VERSION                 = "v1"
	API_NAME                    = "apiName"
	APP_GATE_WAY                = "app-gateway"
	APP_USERS_SERVICE           = "app-users"
	RABBITMQ_URI                = "RABBITMQ_URI"
	MONGO_INITDB_ROOT_USERNAME  = "MONGO_INITDB_ROOT_USERNAME"
	MONGO_INITDB_ROOT_PASSWORD  = "MONGO_INITDB_ROOT_PASSWORD"
	MONGO_HOST                  = "MONGO_HOST"
	REDIS_URL                   = "REDIS_URL"
	REDIS_PASSWORD              = "REDIS_PASSWORD"
	CURRENT_ENV                 = "CURRENT_ENV"
	DEV_ENV                     = "DEV"
	PROD_ENV                    = "PROD"
	ALLOW_ORIGIN_URL_LOCAL_HOST = "http://127.0.0.1:3000/*"
	ALLOW_ORIGIN_URL_CLUSTER    = "myk8sapptest.com/*"
	HOST_PORT                   = "HOST_PORT"
	CSS_STYLE_PATH              = "/public/css/style.css"
	JS_INDEX_PATH               = "/public/js/index.js"
	LOCAL_HOST                  = "localhost"
	MQTT_BROKER_URL             = "MQTT_BROKER_URL"
	MQTT_BROKER_URL_TLS         = "MQTT_BROKER_URL_TLS"
	MQTT_USER                   = "MQTT_USER"
	MQTT_PASSWORD               = "MQTT_PASSWORD"
)

const (
	HEADER_APPLICATION_HTML                  = "text/html"
	HEADER_USER_AGENT                        = "User-Agent"
	HEADER_CONTENT_TYPE                      = "Content-Type"
	HEADER_APPLICATION_JSON                  = "application/json"
	HEADER_TOKEN                             = "Token"
	HEADER_APPLICATION_CSS                   = "text/css"
	HEADER_AUTHORIZATION                     = "Authorization"
	HEADER_COOKIE                            = "Cookie"
	HEADER_HX_LOCATION                       = "HX-Location"
	HEADER_HX_RETARGET                       = "HX-Retarget"
	HEADER_HX_RESWAP                         = "HX-Reswap"
	HEADER_HX_REQUEST                        = "Hx-Request"
	HEADER_CORS_ACCESS_CONTROL_ALLOW_ORIGIN  = "Access-Control-Allow-Origin"
	HEADER_CORS_ACCESS_CONTROL_ALLOW_METHODS = "Access-Control-Allow-Methods"
	HEADER_CORS_ACCESS_CONTROL_ALLOW_HEADERS = "Access-Control-Allow-Headers"
)

// /api/v1/
var API_ENDPOINT = fmt.Sprintf("/%s/%s/", API, API_VERSION)
