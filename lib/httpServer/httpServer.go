package httpServer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
)

type MiddleWareOptions func(ctx ICtx, log logger.ILogger, w http.ResponseWriter, r *http.Request) error
type HttpHandler func(http.ResponseWriter, *http.Request)
type HttpHandlerFunc map[string]func(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler
type Routes map[string]string
type SetOptions func(*Server)

type ReturnCtxWithCancel struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

type ReturnCtx struct {
	Ctx context.Context
}

type Ctx struct {
	ctx    context.Context
	lock   sync.RWMutex
	ctxMap map[string]context.Context
}

func NewCtx(ctx context.Context) *Ctx {
	return &Ctx{
		ctx:    ctx,
		ctxMap: make(map[string]context.Context),
	}
}

func (c *Ctx) GetCtx(key string) (*ReturnCtx, error) {
	if ctx, exists := c.ctxMap[key]; exists {
		return &ReturnCtx{
			Ctx: ctx,
		}, nil
	}
	return nil, errors.New("context does not exist")
}

func (c *Ctx) GetCtxValue(key string) any {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if value := c.ctxMap[key]; value != nil {
		return value.Value(key)
	}
	return nil
}

func (c *Ctx) SetCtxValue(key string, value any) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, exists := c.ctxMap[key]; !exists {
		c.ctxMap[key] = context.WithValue(c.ctx, key, value)
		return
	}
	c.ctxMap[key] = context.WithValue(c.ctxMap[key], key, value)
}

func (c *Ctx) WithTimeout(key string, timeOut time.Duration) (*ReturnCtxWithCancel, error) {
	if ctx, exists := c.ctxMap[key]; exists {
		ctxTimeOut, cancel := context.WithTimeout(ctx, timeOut)
		return &ReturnCtxWithCancel{
			Ctx:    ctxTimeOut,
			Cancel: cancel,
		}, nil
	}
	return nil, fmt.Errorf("no context found...")
}

// func (c *Ctx) GetCtxValue(key string) any {
// 	c.lock.RLock()
// 	defer c.lock.RUnlock()
// 	return c.ctxMap[key]
// }

// func (c *Ctx) SetCtxValue(key string, value any) {
// 	c.lock.Lock()
// 	defer c.lock.Unlock()
// 	c.ctxMap[key] = value
// }

// func (c *Ctx) WithTimeout(timeOut time.Duration) (*ReturnCtx, error) {
// 	ctxTimeOut, cancel := context.WithTimeout(c.ctx, timeOut)
// 	return &ReturnCtx{
// 		Ctx:    ctxTimeOut,
// 		Cancel: cancel,
// 	}, nil
// }

func (c *Ctx) SetCookies(w http.ResponseWriter, key, value string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cookie := &http.Cookie{
		Name:     key,
		Value:    value,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	if utils.GetCurrentENV() == types.PROD_ENV {
		cookie.Expires = time.Now().Add(time.Hour)
	}
	http.SetCookie(w, cookie)
}

func (c *Ctx) GetCookie(name string) (*http.Cookie, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cookie, err := http.ParseSetCookie(name)
	if err != nil {
		return nil, err
	}
	return cookie, nil
}

type Server struct {
	ctx        ICtx
	logger     logger.ILogger
	server     *http.Server
	getCtxLock *sync.RWMutex
	mux        *http.ServeMux
	cors       map[string][]string
}

func New(ctx context.Context, logger logger.ILogger, addr int) *Server {
	mux := http.NewServeMux()
	s := Server{
		ctx:        NewCtx(ctx),
		server:     &http.Server{Addr: fmt.Sprintf(":%d", addr), Handler: mux},
		logger:     logger,
		getCtxLock: &sync.RWMutex{},
		mux:        mux,
		cors: map[string][]string{
			types.HEADER_CORS_ACCESS_CONTROL_ALLOW_ORIGIN:  {types.ALLOW_ORIGIN_URL_LOCAL_HOST, types.ALLOW_ORIGIN_URL_CLUSTER},
			types.HEADER_CORS_ACCESS_CONTROL_ALLOW_HEADERS: {types.HEADER_AUTHORIZATION, types.HEADER_APPLICATION_CSS, types.HEADER_APPLICATION_HTML, types.HEADER_APPLICATION_JSON, types.HEADER_CONTENT_TYPE, types.HEADER_HX_LOCATION, types.HEADER_HX_REQUEST, types.HEADER_HX_RESWAP, types.HEADER_HX_RETARGET, types.HEADER_COOKIE},
			types.HEADER_CORS_ACCESS_CONTROL_ALLOW_METHODS: {http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		},
	}
	s.ping()
	s.setLogLevel()
	return &s
}

func (s *Server) SetCore(rules map[string][]string) {
	for key, newValue := range rules {
		if currentValue, exists := s.cors[key]; !exists {
			s.cors[key] = newValue
		} else {
			currentValue = append(currentValue, newValue...)
		}
	}
}

func (s *Server) GetCtx() ICtx {
	s.getCtxLock.RLock()
	defer s.getCtxLock.RUnlock()
	return s.ctx
}

func (s *Server) SetOptions(opts ...SetOptions) {
	for _, opt := range opts {
		opt(s)
	}
}

// func (s *Server) SetRoutes(routes Routes) SetOptions {
// 	return func(s *Server) {
// 		if routes == nil {
// 			return
// 		}
// 		s.routesLock.Lock()
// 		defer s.routesLock.Unlock()
// 		s.routes = routes
// 	}
// }

// Takes a map[string]string where key is HttpMethod and value is endpoint
func (s *Server) SetGlobalMiddleWare(routes Routes, handler HttpHandler, middleWare ...MiddleWareOptions) {
}

func (s *Server) ListenAndServe() error {
	if err := s.server.ListenAndServe(); err != nil && err == http.ErrServerClosed {
		s.logger.LogErrorf("Lib::ListenAndServe()::error: %+v", err.Error())
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctc context.Context) {
	if err := s.server.Shutdown(ctc); err != nil {
		s.logger.LogErrorf("Lib::Shutdown()::received error::%+v", err.Error())
	}
}

func (s *Server) Handler(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		for key, value := range s.cors {
			w.Header().Set(key, strings.Join(value, ""))
		}
		if len(middleWare) > 0 {
			for _, mid := range middleWare {
				if err := mid(s.ctx, s.logger, w, r); err != nil {
					s.logger.LogErrorf("Lib::Handler()::received error::%+v", err.Error())
					errorMessage := utils.BuildHttpError(err, "User unauthorized to access this page", r.UserAgent(), r.Host)
					errorBytes, _ := utils.JsonMarshall(errorMessage)
					w.WriteHeader(http.StatusUnauthorized)
					w.Write(errorBytes)
					return
				}
			}
		}
		handler(w, r)
	}
}

func (s *Server) Get(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	s.mux.HandleFunc(fmt.Sprintf("%s %s", http.MethodGet, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) Post(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	s.mux.HandleFunc(fmt.Sprintf("%s %s", http.MethodPost, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) Put(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	s.mux.HandleFunc(fmt.Sprintf("%s %s", http.MethodPut, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) Delete(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	s.mux.HandleFunc(fmt.Sprintf("%s %s", http.MethodDelete, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) ping() {
	s.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get(types.HEADER_USER_AGENT), "kube") {
			s.logger.LogInfo(&logger.LoggerPayload{
				Message: "Main::main()::received health check on",
				Value: map[string]any{
					"host":    r.Host,
					"headers": r.Header,
					"method":  r.Method,
				},
			})
		}
		w.WriteHeader(http.StatusOK)
	})
}

func (s Server) setLogLevel() {
	s.Post(types.LOG_LEVEL_ENDPOINT, func(w http.ResponseWriter, r *http.Request) {
		var logLevelReq models.LogLevelRequest
		reqBytes, err := utils.ReadRequestBody(r)
		if err != nil {
			s.logger.LogErrorf("Lib::setLogLevel()::error reading request body::%+v", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			errorMessage := utils.BuildHttpError(err, "Invalid request body", r.UserAgent(), r.Host)
			errorBytes, _ := utils.JsonMarshall(errorMessage)
			w.Write(errorBytes)
			return
		}
		err = logLevelReq.UnMarshal(reqBytes)
		if err != nil {
			s.logger.LogErrorf("Lib::setLogLevel()::error unmarshalling request body::%+v", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			errorMessage := utils.BuildHttpError(err, "Invalid request body", r.UserAgent(), r.Host)
			errorBytes, _ := utils.JsonMarshall(errorMessage)
			w.Write(errorBytes)
			return
		}
		s.logger.SetLogLevel(logLevelReq.LogLevel)

		var response models.LogLevelResponse = models.LogLevelResponse{
			Result:   types.ResponsePayloadResults(types.SUCCESS),
			LogLevel: logLevelReq.LogLevel,
		}
		responseBytes, _ := utils.JsonMarshall(response)
		w.Header().Set(types.HEADER_CONTENT_TYPE, types.HEADER_APPLICATION_JSON)
		w.WriteHeader(http.StatusOK)
		w.Write(responseBytes)
	})
}
