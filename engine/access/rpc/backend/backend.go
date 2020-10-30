package backend

import (
	"context"
	"errors"
	"fmt"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

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

	executionRPC execproto.ExecutionAPIClient
	state        protocol.State
	chainID      flow.ChainID
	collections  storage.Collections
}

func New(
	state protocol.State,
	executionRPC execproto.ExecutionAPIClient,
	collectionRPC accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	chainID flow.ChainID,
	transactionMetrics module.TransactionMetrics,
	collectionGRPCPort uint,
	connFactory ConnectionFactory,
	retryEnabled bool,
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
			headers:      headers,
			executionRPC: executionRPC,
			state:        state,
		},
		backendTransactions: backendTransactions{
			staticCollectionRPC:  collectionRPC,
			executionRPC:         executionRPC,
			state:                state,
			chainID:              chainID,
			collections:          collections,
			blocks:               blocks,
			transactions:         transactions,
			transactionValidator: configureTransactionValidator(state, chainID),
			transactionMetrics:   transactionMetrics,
			retry:                retry,
			collectionGRPCPort:   collectionGRPCPort,
			connFactory:          connFactory,
		},
		backendEvents: backendEvents{
			executionRPC: executionRPC,
			state:        state,
			blocks:       blocks,
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
			executionRPC: executionRPC,
			state:        state,
			headers:      headers,
		},
		collections: collections,
		chainID:     chainID,
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
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
