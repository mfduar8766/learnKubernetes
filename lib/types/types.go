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
	API                        = "api"
	API_VERSION                = "v1"
	USER_AGENT                 = "User-Agent"
	CONTENT_TYPE               = "Content-Type"
	APPLICATION_JSON           = "application/json"
	HEADER_TOKEN               = "Token"
	API_NAME                   = "apiName"
	APPLICATION_HTML           = "text/html"
	APP_GATE_WAY               = "app-gateway"
	APP_USERS_SERVICE          = "app-users"
	RABBITMQ_URI               = "RABBITMQ_URI"
	MONGO_INITDB_ROOT_USERNAME = "MONGO_INITDB_ROOT_USERNAME"
	MONGO_INITDB_ROOT_PASSWORD = "MONGO_INITDB_ROOT_PASSWORD"
	MONGO_HOST                 = "MONGO_HOST"
	REDIS_URL                  = "REDIS_URL"
	REDIS_PASSWORD             = "REDIS_PASSWORD"
	CURRENT_ENV                = "CURRENT_ENV"
	DEV_ENV                    = "DEV"
	PROD_ENV                   = "PROD"
	APPLICATION_CSS            = "text/css"
)

// /api/v1/
var API_ENDPOINT = fmt.Sprintf("/%s/%s/", API, API_VERSION)
