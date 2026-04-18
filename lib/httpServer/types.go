package httpServer

import (
	"context"
	"net/http"
	"time"
)

type ICtx interface {
	GetCtx(key string) (*ReturnCtx, error)
	GetCtxValue(key string) any
	SetCtxValue(key string, value any)
	WithTimeout(string, time.Duration) (*ReturnCtxWithCancel, error)
	// WithTimeout(timeOut time.Duration) (*ReturnCtx, error)
	SetCookies(w http.ResponseWriter, key, value string)
	GetCookie(name string) (*http.Cookie, error)
}

type IServer interface {
	GetCtx() ICtx
	SetCore(rules map[string][]string)
	ListenAndServe() error
	Shutdown(ctx context.Context)
	SetOptions(opts ...SetOptions)
	Get(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Post(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Put(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Delete(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Handler(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler
}
