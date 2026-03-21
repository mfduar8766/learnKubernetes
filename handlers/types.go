package handlers

import (
	"net/http"

	"github.com/a-h/templ"
)

type IRequestHandler interface {
	GetRouteData(routeName string) *RoutData
	ProcessRequests(w http.ResponseWriter, r *http.Request)
	Subscribe(topics ...string)
	ClearSubscriptions()
	// If you use RenderView in your server, add it here too
	RenderView(w http.ResponseWriter, r *http.Request, data *RoutData, component templ.Component)
}
