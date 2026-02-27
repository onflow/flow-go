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
}, {
	Method:  http.MethodGet,
	Pattern: "/scheduled",
	Name:    "getScheduledTransactions",
	Handler: routes.GetScheduledTransactions,
}, {
	Method:  http.MethodGet,
	Pattern: "/scheduled/transaction/{id}",
	Name:    "getScheduledTransaction",
	Handler: routes.GetScheduledTransaction,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/scheduled",
	Name:    "getScheduledTransactionsByAddress",
	Handler: routes.GetScheduledTransactionsByAddress,
}, {
	Method:  http.MethodGet,
	Pattern: "/contracts",
	Name:    "getContracts",
	Handler: routes.GetContracts,
}, {
	Method:  http.MethodGet,
	Pattern: "/contracts/account/{address}",
	Name:    "getContractsByAddress",
	Handler: routes.GetContractsByAddress,
}, {
	Method:  http.MethodGet,
	Pattern: "/contracts/{identifier}",
	Name:    "getContract",
	Handler: routes.GetContract,
}, {
	Method:  http.MethodGet,
	Pattern: "/contracts/{identifier}/deployments",
	Name:    "getContractDeployments",
	Handler: routes.GetContractDeployments,
}}
