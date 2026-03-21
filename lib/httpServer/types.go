package httpServer

import (
	"context"
	"time"
)

type ICtx interface {
	GetCtxValue(key string) any
	SetCtxValue(key string, value any)
	WithTimeout(timeOut time.Duration) (*ReturnCtx, error)
}

type IServer interface {
	GetCtx() ICtx // Returns the ICtx interface defined above
	ListenAndServe() error
	Shutdown(ctx context.Context)
	SetOptions(opts ...SetOptions)

	Get(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Post(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Put(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Delete(pattern string, handler HttpHandler, middleWare ...MiddleWareOptions)
	Handler(handler HttpHandler, middleWare ...MiddleWareOptions) HttpHandler
}
