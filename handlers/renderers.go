package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/views"
)

func (rh *RequestHandler) RenderView(w http.ResponseWriter, r *http.Request, routeData *RoutData, children templ.Component) {
	w.Header().Set(types.HEADER_CONTENT_TYPE, types.HEADER_APPLICATION_HTML)
	if r.Header.Get(types.HEADER_HX_REQUEST) == "true" {
		err := children.Render(r.Context(), w)
		rh.onError(w, r, err, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctxWithChildren := templ.WithChildren(r.Context(), children)
	err := views.Index(routeData.Path, routeData.StyleSheet, routeData.JsFilePath).Render(ctxWithChildren, w)
	rh.onError(w, r, err, "Internal server error", http.StatusInternalServerError)
}

func (rh *RequestHandler) onError(w http.ResponseWriter, r *http.Request, err error, msg string, code int) {
	if err != nil {
		http.Error(w, msg, code)
		views.PageNotFound().Render(r.Context(), w)
	}
}
