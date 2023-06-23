package api

import (
	"context"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
)

// RestServerApi is the server API for REST service.
type RestServerApi interface {
	// GetTransactionByID gets a transaction by requested ID.
	GetTransactionByID(r request.GetTransaction, context context.Context, link models.LinkGenerator, chain flow.Chain) (models.Transaction, error)
	// CreateTransaction creates a new transaction from provided payload.
	CreateTransaction(r request.CreateTransaction, context context.Context, link models.LinkGenerator) (models.Transaction, error)
	// GetTransactionResultByID retrieves transaction result by the transaction ID.
	GetTransactionResultByID(r request.GetTransactionResult, context context.Context, link models.LinkGenerator) (models.TransactionResult, error)
	// GetBlocksByIDs gets blocks by provided ID or list of IDs.
	GetBlocksByIDs(r request.GetBlockByIDs, context context.Context, expandFields map[string]bool, link models.LinkGenerator) ([]*models.Block, error)
	// GetBlocksByHeight gets blocks by provided height.
	GetBlocksByHeight(r *request.Request, link models.LinkGenerator) ([]*models.Block, error)
	// GetBlockPayloadByID gets block payload by ID
	GetBlockPayloadByID(r request.GetBlockPayload, context context.Context, link models.LinkGenerator) (models.BlockPayload, error)
	// GetExecutionResultByID gets execution result by the ID.
	GetExecutionResultByID(r request.GetExecutionResult, context context.Context, link models.LinkGenerator) (models.ExecutionResult, error)
	// GetExecutionResultsByBlockIDs gets Execution Result payload by block IDs.
	GetExecutionResultsByBlockIDs(r request.GetExecutionResultByBlockIDs, context context.Context, link models.LinkGenerator) ([]models.ExecutionResult, error)
	// GetCollectionByID retrieves a collection by ID and builds a response
	GetCollectionByID(r request.GetCollection, context context.Context, expandFields map[string]bool, link models.LinkGenerator, chain flow.Chain) (models.Collection, error)
	// ExecuteScript handler sends the script from the request to be executed.
	ExecuteScript(r request.GetScript, context context.Context, link models.LinkGenerator) ([]byte, error)
	// GetAccount handler retrieves account by address and returns the response.
	GetAccount(r request.GetAccount, context context.Context, expandFields map[string]bool, link models.LinkGenerator) (models.Account, error)
	// GetEvents for the provided block range or list of block IDs filtered by type.
	GetEvents(r request.GetEvents, context context.Context) (models.BlocksEvents, error)
	// GetNetworkParameters returns network-wide parameters of the blockchain
	GetNetworkParameters(r *request.Request) (models.NetworkParameters, error)
	// GetNodeVersionInfo returns node version information
	GetNodeVersionInfo(r *request.Request) (models.NodeVersionInfo, error)
}
