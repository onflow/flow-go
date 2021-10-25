package swagger

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func NewRestAPIServer(api *RestAPI, listenAddress string) *http.Server {

	router := mux.NewRouter().StrictSlash(true)
	for _, route := range apiRoutes(api) {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)
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

func apiRoutes(api *RestAPI) Routes {
	return Routes{
		Route{
			"BlocksIdGet",
			strings.ToUpper("Get"),
			"/v1/blocks/{id}",
			api.BlocksIdGet,
		},
	}
}
