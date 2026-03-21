package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/mfduar8766/learnKubernetes/views"
)

func (rh *RequestHandler) RenderView(w http.ResponseWriter, r *http.Request, routeData *RoutData, children templ.Component) {
	if r.Header.Get("Hx-Request") == "true" {
		err := children.Render(r.Context(), w)
		rh.onError(w, r, err, "Internal server error", http.StatusInternalServerError)
		return
	}
	// 2. Wrap the existing request context with the children (the view)
	ctxWithChildren := templ.WithChildren(r.Context(), children)

	// 3. Render the Layout using the context that now contains the children
	// This allows the { children... } block in your templ file to find 'view'
	err := views.Index(routeData.Path, routeData.StyleSheet).Render(ctxWithChildren, w)
	rh.onError(w, r, err, "Internal server error", http.StatusInternalServerError)
}

// func (rh *RequestHandler) renderHomePage(w http.ResponseWriter, r *http.Request, users *[]models.UserModel) {
// 	err := views.GetUser(*users).Render(r.Context(), w)
// 	rh.onError(w, r, err, "Error getting home page...", http.StatusInternalServerError)
// 	rh.RenderView(w, r, views.Home())
// }

func (rh *RequestHandler) onError(w http.ResponseWriter, r *http.Request, err error, msg string, code int) {
	if err != nil {
		http.Error(w, msg, code)
		views.PageNotFound().Render(r.Context(), w)
	}
}
