package extended

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

type ContractDeploymentExpandOptions struct {
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
	searchSuffix := "." + f.ContractName
	return func(d *accessmodel.ContractDeployment) bool {
		if f.ContractName != "" && strings.HasSuffix(d.ContractID, searchSuffix) {
			return true
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
	deployment, err := b.store.ByContractID(id)
	if err != nil {
		return nil, b.mapReadError(ctx, "contract", err)
	}

	if expandOptions.HasExpand() {
		if err := b.expand(ctx, &deployment, expandOptions, encodingVersion); err != nil {
			err = fmt.Errorf("failed to expand contract deployment %s: %w", deployment.ContractID, err)
			irrecoverable.Throw(ctx, err)
			return nil, err
		}
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
	cursor *accessmodel.ContractDeploymentCursor,
	filter ContractDeploymentFilter,
	expandOptions ContractDeploymentExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	iter, err := b.store.DeploymentsByContractID(id, cursor)
	if err != nil {
		return nil, b.mapReadError(ctx, "contract deployments", err)
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
			if err := b.expand(ctx, &page.Deployments[i], expandOptions, encodingVersion); err != nil {
				err = fmt.Errorf("failed to expand contract deployment %s: %w", page.Deployments[i].ContractID, err)
				irrecoverable.Throw(ctx, err)
				return nil, err
			}
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
	cursor *accessmodel.ContractDeploymentCursor,
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
		return nil, b.mapReadError(ctx, "contracts", err)
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
			if err := b.expand(ctx, &page.Deployments[i], expandOptions, encodingVersion); err != nil {
				err = fmt.Errorf("failed to expand contract deployment %s: %w", page.Deployments[i].ContractID, err)
				irrecoverable.Throw(ctx, err)
				return nil, err
			}
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
	cursor *accessmodel.ContractDeploymentCursor,
	filter ContractDeploymentFilter,
	expandOptions ContractDeploymentExpandOptions,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	iter, err := b.store.ByAddress(address, cursor)
	if err != nil {
		return nil, b.mapReadError(ctx, "contracts", err)
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
			if err := b.expand(ctx, &page.Deployments[i], expandOptions, encodingVersion); err != nil {
				err = fmt.Errorf("failed to expand contract deployment %s: %w", page.Deployments[i].ContractID, err)
				irrecoverable.Throw(ctx, err)
				return nil, err
			}
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
