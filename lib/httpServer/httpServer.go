package httpServer

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

type MiddleWareOptions func(ctx context.Context, logger *slog.Logger, headers http.Header) error
type HttpHandler func(http.ResponseWriter, *http.Request)

// type HttpHandleFunc func(pattern string, handler HttpHandler)
type HttpHandlerFunc map[string]func(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler
type Routes map[string]string
type SetOptions func(*Server)

type Server struct {
	ctx         context.Context
	logger      *slog.Logger
	server      *http.Server
	handlers    HttpHandlerFunc
	routes      Routes
	routesLock  *sync.Mutex
	handlerLock *sync.Mutex
}

func NewServer(ctx context.Context, logger *slog.Logger, addr string) *Server {
	return &Server{
		ctx:         ctx,
		server:      &http.Server{Addr: addr, Handler: http.DefaultServeMux},
		logger:      logger,
		handlers:    make(HttpHandlerFunc),
		routes:      make(Routes),
		routesLock:  &sync.Mutex{},
		handlerLock: &sync.Mutex{},
	}
}

func (s *Server) SetOptions(opts ...SetOptions) {
	for _, opt := range opts {
		opt(s)
	}
}

func (s *Server) SetRoutes(routes Routes) SetOptions {
	return func(s *Server) {
		if routes == nil {
			return
		}
		s.routesLock.Lock()
		defer s.routesLock.Unlock()
		s.routes = routes
	}
}

// Takes a map[string]string where key is HttpMethod and value is endpoint
func (s *Server) SetGlobalMiddleWare(routes Routes, handler HttpHandler, middleWare ...MiddleWareOptions) {

}

func (s *Server) ListenAndServe() error {
	if err := s.server.ListenAndServe(); err != nil && err == http.ErrServerClosed {
		s.logger.Error("Lib::ListenAndServe()::serverClosed", "error", err.Error())
		return err
	}
	return nil
}

func (s *Server) Handler(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(middleWare) > 0 {
			for _, mid := range middleWare {
				if err := mid(s.ctx, s.logger, r.Header); err != nil {
					s.logger.Error("Lib::Handler()::received error", "error", err.Error())
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

type MiddleWare struct {
	ctx    context.Context
	logger *slog.Logger
}

func NewMiddleWare(ctx context.Context, logger *slog.Logger) *MiddleWare {
	return &MiddleWare{
		ctx:    ctx,
		logger: logger,
	}
}

func (m *MiddleWare) Handler(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(middleWare) > 0 {
			for _, mid := range middleWare {
				if err := mid(m.ctx, m.logger, r.Header); err != nil {
					m.logger.Error("Lib::Handler()::received error", "error", err.Error())
					w.WriteHeader(http.StatusUnauthorized)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		handler(w, r)
	}
}
