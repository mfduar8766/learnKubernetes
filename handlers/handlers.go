package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/mfduar8766/learnKubernetes/lib/events"
	"github.com/mfduar8766/learnKubernetes/lib/httpServer"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/lib/rmq"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
)

type RequestHandler struct {
	logger        *logger.Logger
	Broker        *rmq.RMQ
	QueueNames    map[string]string
	ExchangeNames map[string]string
	ctx           *httpServer.Ctx
	subScribeLock *sync.Mutex
}

func NewHandler(ctx *httpServer.Ctx, broker *rmq.RMQ, log *logger.Logger) *RequestHandler {
	return &RequestHandler{
		ctx:           ctx,
		Broker:        broker,
		QueueNames:    map[string]string{rmq.USERS_QUEUE: rmq.USERS_QUEUE},
		ExchangeNames: map[string]string{rmq.USERS_EX: rmq.USERS_EX},
		logger:        log,
		subScribeLock: &sync.Mutex{},
	}
}

func (h *RequestHandler) ProcessRequests(w http.ResponseWriter, r *http.Request) {
	var apiName string = r.Header.Get(types.API_NAME)
	switch apiName {
	case string(events.GET_POSTS):
		h.getPosts(w, r)
	case string(events.GET_USERS):
		h.getUsers(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (rh *RequestHandler) getUsers(w http.ResponseWriter, r *http.Request) {
	rh.logger.LogInfo(&logger.LoggerPayload{
		Message:  "Received getUsers",
		Method:   "GetUsers()",
		FileName: "handlers.go",
	})
	request := models.CreateNewMessagePayload(events.UserEvents(events.GET_USERS), &models.Params{}, nil, nil)
	requstBytes, err := request.Marshall()
	if err != nil {
		rh.logger.LogError(&logger.LoggerPayload{
			Message: "Error marshalling message payload",
			Value:   err,
		})
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response, err := rh.Broker.PubSub(rh.Broker.BuildTopic(rh.ExchangeNames[rmq.USERS_EX], rh.QueueNames[rmq.USERS_QUEUE], rmq.Request), requstBytes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		request.Error = utils.BuildHttpError(err, "failed to get users data", r.UserAgent(), r.Host)
		errorBytes, err := request.Marshall()
		utils.HandleError(err, "GetUsers()", "cannot get users at this time", rh.logger)
		w.Write(errorBytes)
		return
	}
	if response.Payload == nil {
		request.Error = utils.BuildHttpError(nil, "no users found", r.UserAgent(), r.Host)
		errorBytes, err := request.Marshall()
		utils.HandleError(err, "GetUsers()", "cannot get users at this time", rh.logger)
		w.Write(errorBytes)
	}
	rh.logger.LogInfo(&logger.LoggerPayload{
		Message: "Received response from service:",
		Value: map[string]string{
			"service": response.Service,
			"payload": string(response.Payload),
		},
		Method:   "GetUsers()",
		FileName: "handlers.go",
	})
	w.Write(response.Payload)
}

func (rh *RequestHandler) getPosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(types.CONTENT_TYPE, types.APPLICATION_JSON)

	postBytes, err := os.ReadFile("posts.json")
	if err != nil {
		http.Error(w, "File not found", http.StatusInternalServerError)
		return
	}

	var response models.Request[[]models.Posts]
	err = json.Unmarshal(postBytes, &response.Body)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusInternalServerError)
		return
	}
	response.MetaData = map[string]any{
		types.HEADER_TOKEN: rh.ctx.GetCtxValue(types.HEADER_TOKEN),
	}
	// 2. Set the status code SECOND
	w.WriteHeader(http.StatusOK)

	// 3. Write the body LAST
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}

func (rh *RequestHandler) Subscribe(topics ...string) {
	for _, topic := range topics {
		rh.Broker.Subscribe(topic)
	}
}

func (rh *RequestHandler) ClearSubscriptions() {
	rh.Broker.ClearSubscriptions()
}
