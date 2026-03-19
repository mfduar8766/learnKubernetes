package httpServer

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/types"
)

type MiddleWareOptions func(ctx *Ctx, log *logger.Logger, headers http.Header) error
type HttpHandler func(http.ResponseWriter, *http.Request)
type HttpHandlerFunc map[string]func(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler
type Routes map[string]string
type SetOptions func(*Server)
type ReturnCtx struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

type Ctx struct {
	ctx    context.Context
	ctxMap map[string]context.Context
	lock   *sync.Mutex
	rLock  *sync.RWMutex
}

func NewCtx(ctx context.Context) *Ctx {
	return &Ctx{
		ctx:    ctx,
		ctxMap: map[string]context.Context{},
		lock:   &sync.Mutex{},
		rLock:  &sync.RWMutex{},
	}
}

func (c *Ctx) GetCtxValue(key string) any {
	c.rLock.RLock()
	defer c.rLock.RUnlock()
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

func (c *Ctx) WithTimeout(key string, timeOut time.Duration) (*ReturnCtx, error) {
	if ctx, exists := c.ctxMap[key]; exists {
		ctxTimeOut, cancel := context.WithTimeout(ctx, timeOut)
		return &ReturnCtx{
			Ctx:    ctxTimeOut,
			Cancel: cancel,
		}, nil
	}
	return nil, fmt.Errorf("no context found...")
}

type Server struct {
	ctx        *Ctx
	logger     *logger.Logger
	server     *http.Server
	getCtxLock *sync.RWMutex
	// handlers    HttpHandlerFunc
	// routes      Routes
	// routesLock  *sync.Mutex
	// handlerLock *sync.Mutex
}

func NewServer(ctx context.Context, logger *logger.Logger, addr int) *Server {
	s := Server{
		ctx:        NewCtx(ctx),
		server:     &http.Server{Addr: fmt.Sprintf(":%d", addr), Handler: http.DefaultServeMux},
		logger:     logger,
		getCtxLock: &sync.RWMutex{},
		// handlers:    make(HttpHandlerFunc),
		// routes:      make(Routes),
		// routesLock:  &sync.Mutex{},
		// handlerLock: &sync.Mutex{},
	}
	s.ping()
	return &s
}

func (s *Server) GetCtx() *Ctx {
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
		if len(middleWare) > 0 {
			for _, mid := range middleWare {
				if err := mid(s.ctx, s.logger, r.Header); err != nil {
					s.logger.LogErrorf("Lib::Handler()::received error::%+v", err.Error())
					w.WriteHeader(http.StatusUnauthorized)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		handler(w, r)
	}
}

func (s *Server) Get(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	// s.logger.LogInfof("Server::Get()::received request on: %s, setting pattern: %s", pattern, fmt.Sprintf("%s %s", http.MethodGet, pattern))
	http.HandleFunc(fmt.Sprintf("%s %s", http.MethodGet, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) Post(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	http.HandleFunc(fmt.Sprintf("%s %s", http.MethodPost, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) Put(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	http.HandleFunc(fmt.Sprintf("%s %s", http.MethodPut, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) Delete(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions) {
	http.HandleFunc(fmt.Sprintf("%s %s", http.MethodDelete, pattern), s.Handler(handler, middleWare...))
}

func (s *Server) ping() {
	s.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get(types.USER_AGENT), "kube") {
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
