package rest

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/generated"
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

// apiRoutes returns the Gorilla Mux routes for each of the API defined in the rest definition
// currently, it only supports BlocksIdGet
func apiRoutes(api *RestAPIHandler) generated.Routes {
	return generated.Routes{
		generated.Route{
			Name:        "AccountsAddressGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/accounts/{address}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/blocks",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "BlocksIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/blocks/{id}",
			HandlerFunc: api.BlocksIdGet,
		},

		generated.Route{
			Name:        "CollectionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/collections/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/execution_results",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ExecutionResultsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/execution_results/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "ScriptsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/v1/scripts",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionResultsTransactionIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/transaction_results/{transaction_id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsIdGet",
			Method:      strings.ToUpper("Get"),
			Pattern:     "/v1/transactions/{id}",
			HandlerFunc: api.NotImplemented,
		},

		generated.Route{
			Name:        "TransactionsPost",
			Method:      strings.ToUpper("Post"),
			Pattern:     "/v1/transactions",
			HandlerFunc: api.NotImplemented,
		},
	}
}
