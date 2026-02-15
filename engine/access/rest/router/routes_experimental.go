package router

import (
	"net/http"

	"github.com/onflow/flow-go/engine/access/rest/experimental"
)

// Route defines an experimental API route.
type experimentalRoute struct {
	Name    string
	Method  string
	Pattern string
	Handler experimental.ApiHandlerFunc
}

// Routes lists all experimental API routes.
var ExperimentalRoutes = []experimentalRoute{{
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/transactions",
	Name:    "getAccountTransactions",
	Handler: experimental.GetAccountTransactions,
}}
