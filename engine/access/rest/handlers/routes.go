package handlers

import "net/http"

type routeDefinition struct {
	Name    string
	Method  string
	Pattern string
	Handler ApiHandlerFunc
}

var Routes = []routeDefinition{
	// Transactions
	{
		Method:  http.MethodGet,
		Pattern: "/transactions/{id}",
		Name:    "getTransactionByID",
		Handler: getTransactionByID,
	}, {
		Method:  http.MethodPost,
		Pattern: "/transactions",
		Name:    "createTransaction",
		Handler: createTransaction,
	},
	// Transaction Results
	{
		Method:  http.MethodGet,
		Pattern: "/transaction_results/{id}",
		Name:    "getTransactionResultByID",
		Handler: getTransactionResultByID,
	},
	// Blocks
	{
		Method:  http.MethodGet,
		Pattern: "/blocks/{id}",
		Name:    "getBlocksByIDs",
		Handler: getBlocksByIDs,
	}, {
		Method:  http.MethodGet,
		Pattern: "/blocks",
		Name:    "getBlocksByHeight",
		Handler: getBlocksByHeight,
	},
	// Block Payload
	{
		Method:  http.MethodGet,
		Pattern: "/blocks/{id}/payload",
		Name:    "getBlockPayloadByID",
		Handler: getBlockPayloadByID,
	},
	// Execution Result
	{
		Method:  http.MethodGet,
		Pattern: "/execution_results/{id}",
		Name:    "getExecutionResultByID",
		Handler: getExecutionResultByID,
	},
	{
		Method:  http.MethodGet,
		Pattern: "/execution_results",
		Name:    "getExecutionResultByBlockID",
		Handler: getExecutionResultsByBlockIDs,
	},
	// Collections
	{
		Method:  http.MethodGet,
		Pattern: "/collections/{id}",
		Name:    "getCollectionByID",
		Handler: getCollectionByID,
	},
	// Scripts
	{
		Method:  http.MethodPost,
		Pattern: "/scripts",
		Name:    "executeScript",
		Handler: executeScript,
	},
	// Accounts
	{
		Method:  http.MethodGet,
		Pattern: "/accounts/{address}",
		Name:    "getAccount",
		Handler: getAccount,
	},
}
