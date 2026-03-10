package extended

import (
	"context"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
)

type ContractDeploymentExpandOptions struct {
	Code        bool
	Transaction bool
	Result      bool
}

func (o *ContractDeploymentExpandOptions) HasExpand() bool {
	return o.Transaction || o.Result
}

// ContractDeploymentFilter specifies optional filter criteria for contract deployment queries.
// All fields are optional; nil/zero fields are ignored.
type ContractDeploymentFilter struct {
	// ContractName is a partial match against the name component of the contract identifier
	// (e.g. "A.{addr}.{name}").
	ContractName string
	// StartBlock is an inclusive block height lower bound.
	StartBlock *uint64
	// EndBlock is an inclusive block height upper bound.
	EndBlock *uint64
}

// Filter builds a [storage.IndexFilter] from the non-nil filter fields.
func (f *ContractDeploymentFilter) Filter() storage.IndexFilter[*accessmodel.ContractDeployment] {
	// No nil filters. Always filter out deleted contracts.
	// When deleting contracts is eventually supported, make this a configurable option. for now,
	// always remove deleted contracts from the results.

	return func(d *accessmodel.ContractDeployment) bool {
		if d.IsDeleted {
			return false
		}
		if f.ContractName != "" && d.ContractName != f.ContractName {
			return false
		}
		if f.StartBlock != nil && d.BlockHeight < *f.StartBlock {
			return false
		}
		if f.EndBlock != nil && d.BlockHeight > *f.EndBlock {
			return false
		}
		return true
	}
}

// ContractsBackend implements the contracts portion of the extended API.
type ContractsBackend struct {
	*backendBase

	log   zerolog.Logger
	store storage.ContractDeploymentsIndexReader
}

// NewContractsBackend creates a new [ContractsBackend].
func NewContractsBackend(
	log zerolog.Logger,
	base *backendBase,
	store storage.ContractDeploymentsIndexReader,
) *ContractsBackend {
	return &ContractsBackend{
		backendBase: base,
		log:         log,
		store:       store,
	}
}

// GetContract returns the most recent deployment of the contract with the given identifier.
//
// Expected error returns during normal operations:
//   - [codes.NotFound]: if no contract with the given identifier exists
//   - [codes.FailedPrecondition]: if the index has not been initialized
func (b *ContractsBackend) GetContract(
	ctx context.Context,
	id string,
	filter ContractDeploymentFilter,
	expandOptions ContractDeploymentExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ContractDeployment, error) {
	account, contractName, err := accessmodel.ParseContractID(id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid contract identifier: %v", err)
	}

	deployment, err := b.store.ByContract(account, contractName)
	if err != nil {
		return nil, mapReadError(ctx, "contract", err)
	}

	if expandOptions.HasExpand() {
		if err := b.expand(ctx, &deployment, expandOptions, encodingVersion); err != nil {
			err = fmt.Errorf("failed to expand contract deployment %s: %w", id, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
	}

	// code is always loaded from storage, so remove it if not requested
	if !expandOptions.Code {
		deployment.Code = nil
	}

	return &deployment, nil
}

// GetContractDeployments returns a paginated list of all deployments of the given contract,
// most recent first (descending by block height, then by TxIndex, then by EventIndex).
//
// Expected error returns during normal operations:
//   - [codes.NotFound]: if no contract with the given identifier exists
//   - [codes.FailedPrecondition]: if the index has not been initialized
//   - [codes.InvalidArgument]: if query parameters are invalid
func (b *ContractsBackend) GetContractDeployments(
	ctx context.Context,
	id string,
	limit uint32,
	cursor *accessmodel.ContractDeploymentsCursor,
	filter ContractDeploymentFilter,
	expandOptions ContractDeploymentExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	account, contractName, err := accessmodel.ParseContractID(id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid contract identifier: %v", err)
	}

	if cursor != nil {
		// ignore address/contract name passed by the caller
		cursor.Address = account
		cursor.ContractName = contractName
	}

	iter, err := b.store.DeploymentsByContract(account, contractName, cursor)
	if err != nil {
		return nil, mapReadError(ctx, "contract deployments", err)
	}

	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter.Filter())
	if err != nil {
		err = fmt.Errorf("error collecting contract deployments: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	page := &accessmodel.ContractDeploymentPage{
		Deployments: collected,
		NextCursor:  nextCursor,
	}

	if expandOptions.HasExpand() {
		for i := range page.Deployments {
			deployment := &page.Deployments[i]
			if err := b.expand(ctx, deployment, expandOptions, encodingVersion); err != nil {
				err = fmt.Errorf("failed to expand contract deployment at height %d: %w", deployment.BlockHeight, err)
				irrecoverable.Throw(ctx, err)
				return nil, err
			}
		}
	}

	// code is always loaded from storage, so remove it if not requested
	if !expandOptions.Code {
		for i := range page.Deployments {
			page.Deployments[i].Code = nil
		}
	}

	return page, nil
}

// GetContracts returns a paginated list of contracts at their latest deployment.
//
// Expected error returns during normal operations:
//   - [codes.FailedPrecondition]: if the index has not been initialized
//   - [codes.InvalidArgument]: if query parameters are invalid
func (b *ContractsBackend) GetContracts(
	ctx context.Context,
	limit uint32,
	cursor *accessmodel.ContractDeploymentsCursor,
	filter ContractDeploymentFilter,
	expandOptions ContractDeploymentExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	iter, err := b.store.All(cursor)
	if err != nil {
		return nil, mapReadError(ctx, "contracts", err)
	}

	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter.Filter())
	if err != nil {
		err = fmt.Errorf("error collecting contracts: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	page := &accessmodel.ContractDeploymentPage{
		Deployments: collected,
		NextCursor:  nextCursor,
	}

	if expandOptions.HasExpand() {
		for i := range page.Deployments {
			deployment := &page.Deployments[i]
			if err := b.expand(ctx, deployment, expandOptions, encodingVersion); err != nil {
				contractID := accessmodel.ContractID(deployment.Address, deployment.ContractName)
				err = fmt.Errorf("failed to expand contract deployment %s: %w", contractID, err)
				irrecoverable.Throw(ctx, err)
				return nil, err
			}
		}
	}

	// code is always loaded from storage, so remove it if not requested
	if !expandOptions.Code {
		for i := range page.Deployments {
			page.Deployments[i].Code = nil
		}
	}

	return page, nil
}

// GetContractsByAddress returns a paginated list of contracts at their latest deployment for
// the given address.
//
// Expected error returns during normal operations:
//   - [codes.FailedPrecondition]: if the index has not been initialized
//   - [codes.InvalidArgument]: if query parameters are invalid
func (b *ContractsBackend) GetContractsByAddress(
	ctx context.Context,
	address flow.Address,
	limit uint32,
	cursor *accessmodel.ContractDeploymentsCursor,
	filter ContractDeploymentFilter,
	expandOptions ContractDeploymentExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	if cursor != nil {
		// ignore any address passed by the caller
		cursor.Address = address
	}

	iter, err := b.store.ByAddress(address, cursor)
	if err != nil {
		return nil, mapReadError(ctx, "contracts", err)
	}

	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter.Filter())
	if err != nil {
		err = fmt.Errorf("error collecting contracts by address: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	page := &accessmodel.ContractDeploymentPage{
		Deployments: collected,
		NextCursor:  nextCursor,
	}

	if expandOptions.HasExpand() {
		for i := range page.Deployments {
			deployment := &page.Deployments[i]
			if err := b.expand(ctx, deployment, expandOptions, encodingVersion); err != nil {
				contractID := accessmodel.ContractID(deployment.Address, deployment.ContractName)
				err = fmt.Errorf("failed to expand contract deployment %s: %w", contractID, err)
				irrecoverable.Throw(ctx, err)
				return nil, err
			}
		}
	}

	// code is always loaded from storage, so remove it if not requested
	if !expandOptions.Code {
		for i := range page.Deployments {
			page.Deployments[i].Code = nil
		}
	}

	return page, nil
}

func (b *ContractsBackend) expand(
	ctx context.Context,
	deployment *accessmodel.ContractDeployment,
	expandOptions ContractDeploymentExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) error {
	header, err := b.headers.ByHeight(deployment.BlockHeight)
	if err != nil {
		return fmt.Errorf("could not retrieve block header: %w", err)
	}

	var isSystemChunkTx bool
	if expandOptions.Transaction {
		var txBody *flow.TransactionBody
		var err error
		txBody, isSystemChunkTx, err = b.getTransactionBody(ctx, header, deployment.TransactionID)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction body: %w", err)
		}
		deployment.Transaction = txBody
	}

	if expandOptions.Result {
		result, err := b.getTransactionResult(ctx, deployment.TransactionID, header, isSystemChunkTx, expandOptions.Transaction, encodingVersion)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction result: %w", err)
		}
		deployment.Result = result
	}

	return nil
}
