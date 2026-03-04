package router

import (
	"net/http"

	"github.com/onflow/flow-go/engine/access/rest/experimental"
	"github.com/onflow/flow-go/engine/access/rest/experimental/routes"
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
	Handler: routes.GetAccountTransactions,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/ft/transfers",
	Name:    "getAccountFungibleTokenTransfers",
	Handler: routes.GetAccountFungibleTokenTransfers,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/nft/transfers",
	Name:    "getAccountNonFungibleTokenTransfers",
	Handler: routes.GetAccountNonFungibleTokenTransfers,
}}
