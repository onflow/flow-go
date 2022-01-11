package handlers

import "net/http"

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
	Handler: getTransactionByID,
}

var CreateTransactionRoute = route{
	Method:  http.MethodPost,
	Pattern: "/transactions",
	Name:    "createTransaction",
	Handler: createTransaction,
}

var GetTransactionResultByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/transaction_results/{id}",
	Name:    "getTransactionResultByID",
	Handler: getTransactionResultByID,
}

var GetBlocksByIDsRoute = route{
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}",
	Name:    "getBlocksByIDs",
	Handler: getBlocksByIDs,
}

var GetBlocksByHeightRoute = route{
	Method:  http.MethodGet,
	Pattern: "/blocks",
	Name:    "getBlocksByHeight",
	Handler: getBlocksByHeight,
}

var GetBlockPayloadByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/blocks/{id}/payload",
	Name:    "getBlockPayloadByID",
	Handler: getBlockPayloadByID,
}

var GetExecutionResultByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/execution_results/{id}",
	Name:    "getExecutionResultByID",
	Handler: getExecutionResultByID,
}

var GetExecutionResultsByBlockIDsRoute = route{
	Method:  http.MethodGet,
	Pattern: "/execution_results",
	Name:    "getExecutionResultByBlockID",
	Handler: getExecutionResultsByBlockIDs,
}

var GetCollectionByIDRoute = route{
	Method:  http.MethodGet,
	Pattern: "/collections/{id}",
	Name:    "getCollectionByID",
	Handler: getCollectionByID,
}

var ExecuteScriptRoute = route{
	Method:  http.MethodPost,
	Pattern: "/scripts",
	Name:    "executeScript",
	Handler: executeScript,
}

var GetAccountRoute = route{
	Method:  http.MethodGet,
	Pattern: "/accounts/{address}",
	Name:    "getAccount",
	Handler: getAccount,
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
