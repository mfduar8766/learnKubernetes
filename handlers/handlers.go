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
	"github.com/mfduar8766/learnKubernetes/lib/transport"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"github.com/mfduar8766/learnKubernetes/views"
	dashboard "github.com/mfduar8766/learnKubernetes/views/dashBoard"
)

// Routes
const (
	INDEX_ROUTE          = "/"
	DASH_BOARD_ROUTE     = "/dash-board"
	PAGE_NOT_FOUND_ROUTE = "/page-not-found"

	USERS_REQUESTS_TOPIC = "users_request"
	USERS_EVENTS_TOPIC   = "users_event"
)

type RoutData struct {
	Path       string
	StyleSheet string
	JsFilePath string
}

type RequestHandler struct {
	logger        logger.ILogger
	broker        transport.ITransport
	ctx           httpServer.ICtx
	subScribeLock sync.RWMutex
	routes        map[string]*RoutData
	topics        map[string]string
}

func New(ctx httpServer.ICtx, broker transport.ITransport, log logger.ILogger) *RequestHandler {
	return &RequestHandler{
		ctx:    ctx,
		broker: broker,
		logger: log,
		routes: map[string]*RoutData{
			INDEX_ROUTE: {
				Path:       INDEX_ROUTE,
				StyleSheet: types.CSS_STYLE_PATH,
				JsFilePath: types.JS_INDEX_PATH,
			},
			DASH_BOARD_ROUTE: {
				Path:       DASH_BOARD_ROUTE,
				StyleSheet: types.CSS_STYLE_PATH,
				JsFilePath: types.JS_INDEX_PATH,
			},
			PAGE_NOT_FOUND_ROUTE: {
				Path:       PAGE_NOT_FOUND_ROUTE,
				StyleSheet: types.CSS_STYLE_PATH,
				JsFilePath: types.JS_INDEX_PATH,
			},
		},
		topics: map[string]string{
			USERS_REQUESTS_TOPIC: broker.BuildTopic(transport.TOPIC_TYPE_REQUEST, "users"),
			USERS_EVENTS_TOPIC:   broker.BuildTopic(transport.TOPIC_TYPE_EVENT, "users"),
		},
	}
}

func (rh *RequestHandler) GetRouteData(routeName string) *RoutData {
	if value, exists := rh.routes[routeName]; exists {
		return value
	}
	return rh.routes[PAGE_NOT_FOUND_ROUTE]
}

func (rh *RequestHandler) ProcessRequests(w http.ResponseWriter, r *http.Request) {
	var apiName string = r.Header.Get(types.API_NAME)
	rh.logger.LogInfof("Handlers::ProcessRequests()::received event: %s", apiName)
	switch apiName {
	case string(events.LOGIN):
		rh.handleLogIn(w, r)
	case string(events.DASH_BOARD):
		// rh.RenderView(w, r, rh.routes[DASH_BOARD_ROUTE], views.Home())
	case string(events.GET_POSTS):
		rh.getPosts(w, r)
	case string(events.GET_USERS):
		rh.getUsers(w, r)
	default:
		w.WriteHeader(http.StatusOK)
		rh.RenderView(w, r, rh.routes[PAGE_NOT_FOUND_ROUTE], views.PageNotFound())
	}
}

func (rh *RequestHandler) handleLogIn(w http.ResponseWriter, r *http.Request) {

	// if !is_authenticated {
	//     warn!("handlers::handleLogIn():email or password is invalid");
	//     let mut h = HeaderMap::new();
	//     h.insert(HEADER_CONTENT_TYPE, "text/html".parse().unwrap());
	//     h.insert("HX-Retarget", "#error-message".parse().unwrap());
	//     h.insert("HX-Reswap", "innerHTML".parse().unwrap());
	//     return (
	//         StatusCode::BAD_REQUEST,
	//         h,
	//         renderers::render_error_message("email or password is invalid. Please try again."),
	//     );
	// }
	// Some validation ETC...

	ctx, cancel := context.WithTimeout(r.Context(), time.Millisecond*100)
	defer cancel()

	request := models.CreateNewMessagePayload[[]*models.UserModel](rh.topics[USERS_REQUESTS_TOPIC], events.UserEvents(events.GET_USERS), &models.Params{}, nil, nil)
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

	responseChan := rh.broker.PublishWithResponse(ctx, rh.topics[USERS_REQUESTS_TOPIC], requestBytes, nil)
	select {
	case <-ctx.Done():
		w.WriteHeader(http.StatusGatewayTimeout)
		request.Error = utils.BuildHttpError(nil, "Request timedout", r.UserAgent(), r.Host)
		errorBytes, err := request.Marshall()
		utils.HandleError(err, "handleLogIn()", "Request timedout. Failed to get response from users service...", rh.logger)
		w.Write(errorBytes)
		return
	case response, ok := <-responseChan:
		if err := transport.CheckTransportResponseForErrors(w, r, request, response, ok); err != nil {
			utils.HandleError(err, "handleLogIn()", "failed logint", rh.logger)
			return
		}
		var users *models.MessagePayload[[]*models.UserModel]
		err = utils.JsonUnMarshall(response.Payload, &users)
		if err != nil {
			request.Error = utils.BuildHttpError(err, "failed to get users", r.UserAgent(), r.Host)
			errorBytes, err := request.Marshall()
			utils.HandleError(err, "handleLogIn()", "failed to get users. Failed to get response from users service...", rh.logger)
			w.WriteHeader(http.StatusNoContent)
			w.Write(errorBytes)
			return
		}
		rh.ctx.SetCookies(w, types.HEADER_TOKEN, "SOME_VALUE")
		w.WriteHeader(http.StatusOK)
		rh.RenderView(w, r, rh.routes[DASH_BOARD_ROUTE], dashboard.DashBoard(users.Response.Data))
	}
}

func (rh *RequestHandler) getUsers(w http.ResponseWriter, r *http.Request) {
	// ctx, cancel := context.WithTimeout(r.Context(), 1000*time.Millisecond)
	// defer cancel()

	// topic := rh.broker.BuildTopic(rh.exchangeNames[rmq.USERS_EX], rh.queueNames[rmq.USERS_QUEUE], rmq.Request)
	// request := models.CreateNewMessagePayload(topic, events.UserEvents(events.GET_USERS), &models.Params{}, nil, nil)
	// requestBytes, err := request.Marshall()
	// if err != nil {
	// 	rh.logger.LogError(&logger.LoggerPayload{
	// 		Message: "Error marshalling message payload",
	// 		Value:   err,
	// 	})
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	// response, err := rh.broker.PubSub(ctx, topic, requestBytes)
	// if err != nil {
	// 	w.WriteHeader(http.StatusGatewayTimeout)
	// 	request.Error = utils.BuildHttpError(err, "request timedout", r.UserAgent(), r.Host)
	// 	errorBytes, err := request.Marshall()
	// 	utils.HandleError(err, "GetUsers()", "request timedout. Failed to get response from users service...", rh.logger)
	// 	w.Write(errorBytes)
	// 	return
	// }

	// w.Header().Set(types.HEADER_CONTENT_TYPE, types.HEADER_APPLICATION_JSON)
	// w.WriteHeader(http.StatusOK)
	// w.Write(response.Payload)
}

func (rh *RequestHandler) getPosts(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(types.HEADER_CONTENT_TYPE, types.HEADER_APPLICATION_JSON)

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

	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}

func (rh *RequestHandler) Subscribe(topics ...string) {
	for _, topic := range topics {
		rh.broker.Subscribe(context.Background(), topic, nil)
	}
}

func (rh *RequestHandler) Unsubscribe(topics ...string) {
	rh.broker.Unsubscribe(context.Background(), topics...)
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
