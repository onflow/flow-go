package swagger

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/swagger/generated"
)

// NewRestAPIServer returns an HTTP server initialized with the REST API handler
func NewRestAPIServer(api *RestAPIHandler, listenAddress string) *http.Server {

	router := mux.NewRouter().StrictSlash(true)
	for _, route := range apiRoutes(api) {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = generated.Logger(handler, route.Name)
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return &http.Server{
		Addr:    listenAddress,
		Handler: router,
	}
}

// apiRoutes returns the Gorilla Mux routes for each of the API defined in the swagger definition
// currently, it only supports BlocksIdGet
func apiRoutes(api *RestAPIHandler) generated.Routes {
	return generated.Routes{
		generated.Route{
			Name:        "BlocksIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/blocks/{id}",
			HandlerFunc: api.BlocksIdGet,
		},
	}
}
