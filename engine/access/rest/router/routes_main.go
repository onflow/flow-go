package router

import (
	"net/http"

	resthttp "github.com/onflow/flow-go/engine/access/rest/http"
	"github.com/onflow/flow-go/engine/access/rest/http/routes"
)

type route struct {
	Name    string
	Method  string
	Pattern string
	Handler resthttp.ApiHandlerFunc
}

var Routes = []route{{
	Method:  http.MethodGet,
	Pattern: "/transactions/{id}",
	Name:    "getTransactionByID",
	Handler: routes.GetTransactionByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/transactions",
	Name:    "getTransactionsByBlock",
	Handler: routes.GetTransactionsByBlock,
}, {
	Method:  http.MethodPost,
	Pattern: "/transactions",
	Name:    "createTransaction",
	Handler: routes.CreateTransaction,
}, {
	Method:  http.MethodGet,
	Pattern: "/transaction_results/{id}",
	Name:    "getTransactionResultByID",
	Handler: routes.GetTransactionResultByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/transaction_results",
	Name:    "getTransactionResultsByBlock",
	Handler: routes.GetTransactionResultsByBlock,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}",
	Name:    "getBlocksByIDs",
	Handler: routes.GetBlocksByIDs,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks",
	Name:    "getBlocksByHeight",
	Handler: routes.GetBlocksByHeight,
}, {
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}/payload",
	Name:    "getBlockPayloadByID",
	Handler: routes.GetBlockPayloadByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/execution_results/{id}",
	Name:    "getExecutionResultByID",
	Handler: routes.GetExecutionResultByID,
}, {
	Method:  http.MethodGet,
	Pattern: "/execution_results",
	Name:    "getExecutionResultByBlockID",
	Handler: routes.GetExecutionResultsByBlockIDs,
}, {
	Method:  http.MethodGet,
	Pattern: "/collections/{id}",
	Name:    "getCollectionByID",
	Handler: routes.GetCollectionByID,
}, {
	Method:  http.MethodPost,
	Pattern: "/scripts",
	Name:    "executeScript",
	Handler: routes.ExecuteScript,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}",
	Name:    "getAccount",
	Handler: routes.GetAccount,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/balance",
	Name:    "getAccountBalance",
	Handler: routes.GetAccountBalance,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/keys/{index}",
	Name:    "getAccountKeyByIndex",
	Handler: routes.GetAccountKeyByIndex,
}, {
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}/keys",
	Name:    "getAccountKeys",
	Handler: routes.GetAccountKeys,
}, {
	Method:  http.MethodGet,
	Pattern: "/events",
	Name:    "getEvents",
	Handler: routes.GetEvents,
}, {
	Method:  http.MethodGet,
	Pattern: "/network/parameters",
	Name:    "getNetworkParameters",
	Handler: routes.GetNetworkParameters,
}, {
	Method:  http.MethodGet,
	Pattern: "/node_version_info",
	Name:    "getNodeVersionInfo",
	Handler: routes.GetNodeVersionInfo,
}}
