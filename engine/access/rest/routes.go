package rest

import (
	"net/http"
)

type route struct {
	Name    string
	Method  string
	Pattern string
	Handler ApiHandlerFunc
}

var GetTransactionByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/transactions/{id}",
	Name:    "getTransactionByID",
	Handler: GetTransactionByID,
}

var CreateTransactionRoute = route{
	Method:  http.MethodPost,
	Pattern: "/transactions",
	Name:    "createTransaction",
	Handler: CreateTransaction,
}

var GetTransactionResultByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/transaction_results/{id}",
	Name:    "getTransactionResultByID",
	Handler: GetTransactionResultByID,
}

var GetBlocksByIDsRoute = route{
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}",
	Name:    "getBlocksByIDs",
	Handler: GetBlocksByIDs,
}

var GetBlocksByHeightRoute = route{
	Method:  http.MethodGet,
	Pattern: "/blocks",
	Name:    "getBlocksByHeight",
	Handler: GetBlocksByHeight,
}

var GetBlockPayloadByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}/payload",
	Name:    "getBlockPayloadByID",
	Handler: GetBlockPayloadByID,
}

var GetExecutionResultByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/execution_results/{id}",
	Name:    "getExecutionResultByID",
	Handler: GetExecutionResultByID,
}

var GetExecutionResultsByBlockIDsRoute = route{
	Method:  http.MethodGet,
	Pattern: "/execution_results",
	Name:    "getExecutionResultByBlockID",
	Handler: GetExecutionResultsByBlockIDs,
}

var GetCollectionByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/collections/{id}",
	Name:    "getCollectionByID",
	Handler: GetCollectionByID,
}

var ExecuteScriptRoute = route{
	Method:  http.MethodPost,
	Pattern: "/scripts",
	Name:    "executeScript",
	Handler: ExecuteScript,
}

var GetAccountRoute = route{
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}",
	Name:    "getAccount",
	Handler: GetAccount,
}

var Routes = []route{
	// Transactions
	GetTransactionByIDRoute,
	CreateTransactionRoute,
	// Transaction Results
	GetTransactionResultByIDRoute,
	// Blocks
	GetBlocksByIDsRoute,
	GetBlocksByHeightRoute,
	// Block Payload
	GetBlockPayloadByIDRoute,
	// Execution Result
	GetExecutionResultByIDRoute,
	GetExecutionResultsByBlockIDsRoute,
	// Collections
	GetCollectionByIDRoute,
	// Scripts
	ExecuteScriptRoute,
	// Accounts
	GetAccountRoute,
}
