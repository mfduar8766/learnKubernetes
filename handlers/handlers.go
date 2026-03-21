package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/events"
	"github.com/mfduar8766/learnKubernetes/lib/httpServer"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/lib/rmq"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"github.com/mfduar8766/learnKubernetes/views"
)

const (
	INDEX          = "index"
	PAGE_NOT_FOUND = "pageNotFound"
)

type RoutData struct {
	StyleSheet string
	Path       string
}

type RequestHandler struct {
	logger        logger.ILogger
	broker        rmq.IRabbitMq
	queueNames    map[string]string
	exchangeNames map[string]string
	ctx           httpServer.ICtx
	subScribeLock *sync.Mutex
	routes        map[string]*RoutData
}

func NewHandler(ctx httpServer.ICtx, broker rmq.IRabbitMq, log logger.ILogger) *RequestHandler {
	return &RequestHandler{
		ctx:           ctx,
		broker:        broker,
		queueNames:    map[string]string{rmq.USERS_QUEUE: rmq.USERS_QUEUE, rmq.POSTS_QUEUE: rmq.POSTS_QUEUE},
		exchangeNames: map[string]string{rmq.USERS_EX: rmq.USERS_EX, rmq.POSTS_EX: rmq.POSTS_EX},
		logger:        log,
		subScribeLock: &sync.Mutex{},
		routes: map[string]*RoutData{
			INDEX: {
				Path:       "/",
				StyleSheet: "/public/css/style.css",
			},
			PAGE_NOT_FOUND: {
				Path:       "*",
				StyleSheet: "/public/css/style.css",
			},
		},
	}
}

func (rh *RequestHandler) GetRouteData(routeName string) *RoutData {
	if value, exists := rh.routes[routeName]; exists {
		return value
	}
	return rh.routes[PAGE_NOT_FOUND]
}

func (rh *RequestHandler) ProcessRequests(w http.ResponseWriter, r *http.Request) {
	var apiName string = r.Header.Get(types.API_NAME)
	switch apiName {
	case string(events.GET_POSTS):
		rh.getPosts(w, r)
	case string(events.GET_USERS):
		rh.getUsers(w, r)
	default:
		w.WriteHeader(http.StatusOK)
		w.Header().Set(types.CONTENT_TYPE, types.APPLICATION_HTML)
		rh.RenderView(w, r, rh.routes[PAGE_NOT_FOUND], views.PageNotFound())
	}
}

func (rh *RequestHandler) getUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1000*time.Millisecond)
	defer cancel()

	topic := rh.broker.BuildTopic(rh.exchangeNames[rmq.USERS_EX], rh.queueNames[rmq.USERS_QUEUE], rmq.Request)
	request := models.CreateNewMessagePayload(topic, events.UserEvents(events.GET_USERS), &models.Params{}, nil, nil)
	requestBytes, err := request.Marshall()
	if err != nil {
		rh.logger.LogError(&logger.LoggerPayload{
			Message: "Error marshalling message payload",
			Value:   err,
		})
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Call PubSub synchronously (the context handles the wait/timeout)
	response, err := rh.broker.PubSub(ctx, topic, requestBytes)
	if err != nil {
		w.WriteHeader(http.StatusGatewayTimeout)
		request.Error = utils.BuildHttpError(err, "request timedout", r.UserAgent(), r.Host)
		errorBytes, err := request.Marshall()
		utils.HandleError(err, "GetUsers()", "request timedout. Failed to get response from users service...", rh.logger)
		w.Write(errorBytes)
		return
	}

	w.Header().Set(types.CONTENT_TYPE, types.APPLICATION_JSON)
	w.WriteHeader(http.StatusOK)
	w.Write(response.Payload)
}

func (rh *RequestHandler) getPosts(w http.ResponseWriter, _ *http.Request) {
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
		rh.broker.Subscribe(topic)
	}
}

func (rh *RequestHandler) ClearSubscriptions() {
	rh.broker.ClearSubscriptions()
}

/*
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	var wait chan struct{} = make(chan struct{})

	go func() {
		select {
		case <-time.After(time.Second * 3):
			fmt.Println("Timed out...")
		case <-ctx.Done():
			fmt.Println("Done called...")
		}
		wait <- struct{}{}
		close(wait)
	}()
	<-wait
	fmt.Println("After wait...")
*/
