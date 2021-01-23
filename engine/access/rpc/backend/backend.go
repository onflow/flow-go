package backend

import (
	"context"
	"errors"
	"fmt"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// maxExecutionNodesCnt is the max number of execution nodes that will be contacted to complete an execution api request
const maxExecutionNodesCnt = 3

// Backends implements the Access API.
//
// It is composed of several sub-backends that implement part of the Access API.
//
// Script related calls are handled by backendScripts.
// Transaction related calls are handled by backendTransactions.
// Block Header related calls are handled by backendBlockHeaders.
// Block details related calls are handled by backendBlockDetails.
// Event related calls are handled by backendEvents.
// Account related calls are handled by backendAccounts.
//
// All remaining calls are handled by the base Backend in this file.
type Backend struct {
	backendScripts
	backendTransactions
	backendEvents
	backendBlockHeaders
	backendBlockDetails
	backendAccounts

	executionRPC      execproto.ExecutionAPIClient
	state             protocol.State
	chainID           flow.ChainID
	collections       storage.Collections
	executionReceipts storage.ExecutionReceipts
	connFactory       ConnectionFactory
}

func New(
	state protocol.State,
	executionRPC execproto.ExecutionAPIClient,
	collectionRPC accessproto.AccessAPIClient,
	historicalAccessNodes []accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionReceipts storage.ExecutionReceipts,
	chainID flow.ChainID,
	transactionMetrics module.TransactionMetrics,
	connFactory ConnectionFactory,
	retryEnabled bool,
	log zerolog.Logger,
) *Backend {
	retry := newRetry()
	if retryEnabled {
		retry.Activate()
	}

	b := &Backend{
		executionRPC: executionRPC,
		state:        state,
		// create the sub-backends
		backendScripts: backendScripts{
			headers:            headers,
			executionReceipts:  executionReceipts,
			staticExecutionRPC: executionRPC,
			connFactory:        connFactory,
			state:              state,
		},
		backendTransactions: backendTransactions{
			staticCollectionRPC:  collectionRPC,
			executionRPC:         executionRPC,
			state:                state,
			chainID:              chainID,
			collections:          collections,
			blocks:               blocks,
			transactions:         transactions,
			executionReceipts:    executionReceipts,
			transactionValidator: configureTransactionValidator(state, chainID),
			transactionMetrics:   transactionMetrics,
			retry:                retry,
			connFactory:          connFactory,
			previousAccessNodes:  historicalAccessNodes,
		},
		backendEvents: backendEvents{
			staticExecutionRPC: executionRPC,
			state:              state,
			blocks:             blocks,
			executionReceipts:  executionReceipts,
			connFactory:        connFactory,
			log:                log,
		},
		backendBlockHeaders: backendBlockHeaders{
			headers: headers,
			state:   state,
		},
		backendBlockDetails: backendBlockDetails{
			blocks: blocks,
			state:  state,
		},
		backendAccounts: backendAccounts{
			staticExecutionRPC: executionRPC,
			state:              state,
			headers:            headers,
			executionReceipts:  executionReceipts,
			connFactory:        connFactory,
		},
		collections:       collections,
		executionReceipts: executionReceipts,
		connFactory:       connFactory,
		chainID:           chainID,
	}

	retry.SetBackend(b)

	return b
}

func configureTransactionValidator(state protocol.State, chainID flow.ChainID) *access.TransactionValidator {
	return access.NewTransactionValidator(
		access.NewProtocolStateBlocks(state),
		chainID.Chain(),
		access.TransactionValidationOptions{
			Expiry:                       flow.DefaultTransactionExpiry,
			ExpiryBuffer:                 flow.DefaultTransactionExpiryBuffer,
			AllowEmptyReferenceBlockID:   false,
			AllowUnknownReferenceBlockID: false,
			MaxGasLimit:                  flow.DefaultMaxGasLimit,
			CheckScriptsParse:            true,
			MaxTxSizeLimit:               flow.DefaultMaxTxSizeLimit,
		},
	)
}

// Ping responds to requests when the server is up.
func (b *Backend) Ping(ctx context.Context) error {
	_, err := b.executionRPC.Ping(ctx, &execproto.PingRequest{})
	if err != nil {
		return fmt.Errorf("could not ping execution node: %w", err)
	}

	// staticCollectionRPC is only set if a collection node address was provided at startup
	if b.staticCollectionRPC != nil {
		_, err = b.staticCollectionRPC.Ping(ctx, &accessproto.PingRequest{})
		if err != nil {
			return fmt.Errorf("could not ping collection node: %w", err)
		}
	}

	return nil
}

func (b *Backend) GetCollectionByID(_ context.Context, colID flow.Identifier) (*flow.LightCollection, error) {
	// retrieve the collection from the collection storage
	col, err := b.collections.LightByID(colID)
	if err != nil {
		err = convertStorageError(err)
		return nil, err
	}

	return col, nil
}

func (b *Backend) GetNetworkParameters(_ context.Context) access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: b.chainID,
	}
}

func convertStorageError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.NotFound {
		// Already converted
		return err
	}
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}

// executionNodesForBlockID returns upto maxExecutionNodesCnt number of randomly chosen execution node identities
// which have executed the given block ID. If no such execution node is found, then an error is returned.
func executionNodesForBlockID(
	blockID flow.Identifier,
	executionReceipts storage.ExecutionReceipts,
	state protocol.State) (flow.IdentityList, error) {

	// lookup the receipts storage with the block ID
	receipts, err := executionReceipts.ByBlockIDAllExecutionReceipts(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution receipts for block ID %v: %w", blockID, err)
	}

	// collect the execution node id in each of the receipts
	var executorIDs flow.IdentifierList
	for _, receipt := range receipts {
		executorIDs = append(executorIDs, receipt.ExecutorID)
	}

	if len(executorIDs) == 0 {
		return nil, fmt.Errorf("no execution node found for block ID %v: %w", blockID, err)
	}

	// find the node identities of these execution nodes
	executionIdentities, err := state.Final().Identities(filter.HasNodeID(executorIDs...))
	if err != nil {
		return nil, fmt.Errorf("failed to retreive execution IDs for block ID %v: %w", blockID, err)
	}

	// randomly choose upto maxExecutionNodesCnt identities
	executionIdentitiesRandom := executionIdentities.Sample(maxExecutionNodesCnt)

	return executionIdentitiesRandom, nil
}
