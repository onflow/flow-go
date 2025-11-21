package provider

import (
	"context"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// AccountProvider defines an abstraction for retrieving Flow account data from a specific block.
//
// Implementations may rely on local storage, execution nodes, or a failover strategy,
// but must return standardized access-layer sentinel errors for consistency.
type AccountProvider interface {
	// GetAccountAtBlock returns a Flow account for the given address and block height.
	//
	// Expected error returns during normal operation:
	//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
	//   - [access.ResourceExhausted]: If computation or memory limits are exceeded.
	//   - [access.DataNotFoundError]: If the data is not found.
	//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
	//   - [access.PreconditionFailedError]: If the registers storage is still bootstrapping.
	//   - [access.RequestCanceledError]: If the request is canceled.
	//   - [access.RequestTimedOutError]: If the request times out.
	//   - [access.InternalError]: For internal failures or index conversion errors.
	GetAccountAtBlock(
		ctx context.Context,
		address flow.Address,
		blockID flow.Identifier,
		height uint64,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*flow.Account, *accessmodel.ExecutorMetadata, error)

	// GetAccountBalanceAtBlock returns the balance of a Flow account
	// at the given block height.
	//
	// Expected error returns during normal operation:
	//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
	//   - [access.ResourceExhausted]: If computation or memory limits are exceeded.
	//   - [access.DataNotFoundError]: If the data is not found.
	//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
	//   - [access.PreconditionFailedError]: If the registers storage is still bootstrapping.
	//   - [access.RequestCanceledError]: If the request is canceled.
	//   - [access.RequestTimedOutError]: If the request times out.
	//   - [access.InternalError]: For internal failures or index conversion errors.
	GetAccountBalanceAtBlock(
		ctx context.Context,
		address flow.Address,
		blockID flow.Identifier,
		height uint64,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (uint64, *accessmodel.ExecutorMetadata, error)

	// GetAccountKeyAtBlock returns a specific public key of a Flow account
	// by its key index and block height.
	//
	// Expected error returns during normal operation:
	//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
	//   - [access.ResourceExhausted]: If computation or memory limits are exceeded.
	//   - [access.DataNotFoundError]: If the data is not found.
	//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
	//   - [access.PreconditionFailedError]: If the registers storage is still bootstrapping.
	//   - [access.RequestCanceledError]: If the request is canceled.
	//   - [access.RequestTimedOutError]: If the request times out.
	//   - [access.InternalError]: For internal failures or index conversion errors.
	GetAccountKeyAtBlock(
		ctx context.Context,
		address flow.Address,
		keyIndex uint32,
		blockID flow.Identifier,
		height uint64,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error)

	// GetAccountKeysAtBlock returns all public keys associated with a Flow account
	// at the given block height.
	//
	// Expected error returns during normal operation:
	//   - [access.InvalidRequestError]: If the request fails due to invalid arguments or runtime errors.
	//   - [access.ResourceExhausted]: If computation or memory limits are exceeded.
	//   - [access.DataNotFoundError]: If the data is not found.
	//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
	//   - [access.PreconditionFailedError]: If the registers storage is still bootstrapping.
	//   - [access.RequestCanceledError]: If the request is canceled.
	//   - [access.RequestTimedOutError]: If the request times out.
	//   - [access.InternalError]: For internal failures or index conversion errors.
	GetAccountKeysAtBlock(
		ctx context.Context,
		address flow.Address,
		blockID flow.Identifier,
		height uint64,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error)
}
