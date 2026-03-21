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

type MiddleWareOptions func(ctx ICtx, log logger.ILogger, w http.ResponseWriter, r *http.Request) error
type HttpHandler func(http.ResponseWriter, *http.Request)
type HttpHandlerFunc map[string]func(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler
type Routes map[string]string
type SetOptions func(*Server)
type ReturnCtx struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

type Ctx struct {
	ctx context.Context
	// We use any (interface{}) to avoid the "Context Chaining" memory leak
	ctxMap map[string]any
	lock   sync.RWMutex // No pointer needed if the struct is passed by ref
}

func NewCtx(ctx context.Context) *Ctx {
	return &Ctx{
		ctx:    ctx,
		ctxMap: make(map[string]any),
		// sync.RWMutex zero-value is ready to use
	}
}

// GetCtxValue now performs a flat O(1) lookup
func (c *Ctx) GetCtxValue(key string) any {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.ctxMap[key]
}

// SetCtxValue now updates the map directly without nesting contexts
func (c *Ctx) SetCtxValue(key string, value any) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.ctxMap[key] = value
}

// WithTimeout derives a child context from the base request context
func (c *Ctx) WithTimeout(timeOut time.Duration) (*ReturnCtx, error) {
	// We don't need a lock here because c.ctx is typically read-only
	ctxTimeOut, cancel := context.WithTimeout(c.ctx, timeOut)

	return &ReturnCtx{
		Ctx:    ctxTimeOut,
		Cancel: cancel,
	}, nil
}

type Server struct {
	ctx        ICtx
	logger     logger.ILogger
	server     *http.Server
	getCtxLock *sync.RWMutex
	mux        *http.ServeMux
}

func NewServer(ctx context.Context, logger logger.ILogger, addr int) *Server {
	mux := http.NewServeMux()
	s := Server{
		ctx:        NewCtx(ctx),
		server:     &http.Server{Addr: fmt.Sprintf(":%d", addr), Handler: mux},
		logger:     logger,
		getCtxLock: &sync.RWMutex{},
		mux:        mux,
	}
	s.ping()
	return &s
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
		if len(middleWare) > 0 {
			for _, mid := range middleWare {
				if err := mid(s.ctx, s.logger, w, r); err != nil {
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
