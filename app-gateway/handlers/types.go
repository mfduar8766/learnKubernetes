package handlers

import (
	"net/http"

	"github.com/a-h/templ"
)

type IRequestHandler interface {
	GetRouteData(routeName string) *RoutData
	ProcessRequests(w http.ResponseWriter, r *http.Request)
	Subscribe(topics ...string)
	Unsubscribe(topics ...string)
	RenderView(w http.ResponseWriter, r *http.Request, data *RoutData, component templ.Component)
}
