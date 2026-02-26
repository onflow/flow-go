package extended

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// ContractDeploymentFilter specifies optional filter criteria for contract deployment queries.
// All fields are optional; nil/zero fields are ignored.
type ContractDeploymentFilter struct {
	// ContractName is a partial match against the name component of the contract identifier
	// (e.g. "A.{addr}.{name}").
	ContractName string
	// StartBlock is an inclusive block height lower bound.
	// TODO: not yet implementable at the storage filter layer since height filtering requires
	// a full scan of deployments.
	StartBlock *uint64
	// EndBlock is an inclusive block height upper bound.
	// TODO: not yet implementable at the storage filter layer.
	EndBlock *uint64
}

// Filter builds a [storage.IndexFilter] from the non-nil filter fields.
func (f *ContractDeploymentFilter) Filter() storage.IndexFilter[*accessmodel.ContractDeployment] {
	return func(d *accessmodel.ContractDeployment) bool {
		if f.ContractName != "" {
			parts := strings.Split(d.ContractID, ".")
			if len(parts) < 3 || !strings.Contains(parts[2], f.ContractName) {
				return false
			}
		}
		// TODO: StartBlock and EndBlock filters are not yet implemented.
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
) (*accessmodel.ContractDeployment, error) {
	deployment, err := b.store.ByContractID(id)
	if err != nil {
		return nil, b.mapReadError(ctx, "contract", err)
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
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	page, err := b.store.DeploymentsByContractID(id, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "contract deployments", err)
	}
	return &page, nil
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
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	page, err := b.store.All(limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "contracts", err)
	}
	return &page, nil
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
) (*accessmodel.ContractDeploymentPage, error) {
	limit, err := b.normalizeLimit(limit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid limit: %v", err)
	}

	page, err := b.store.ByAddress(address, limit, cursor, filter.Filter())
	if err != nil {
		return nil, b.mapReadError(ctx, "contracts", err)
	}
	return &page, nil
}
