package extended

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// makeContractDeployment builds a minimal ContractDeployment for use in backend tests.
func makeContractDeployment(contractID string, height uint64) accessmodel.ContractDeployment {
	addr, _ := flow.StringToAddress(contractID[2:18]) // parse "A.{16hex}.Name"
	return accessmodel.ContractDeployment{
		ContractID:    contractID,
		Address:       addr,
		BlockHeight:   height,
		TransactionID: unittest.IdentifierFixture(),
		Code:          []byte("access(all) contract Foo {}"),
		CodeHash:      make([]byte, 32),
	}
}

// contractSignalerCtxExpectingThrow returns a context and a verify function that asserts
// irrecoverable.Throw was called exactly once. The context is built with
// [irrecoverable.WithSignalerContext] so that [irrecoverable.Throw] can locate the signaler
// via the context value chain.
func contractSignalerCtxExpectingThrow(t *testing.T) (context.Context, func()) {
	t.Helper()
	thrown := make(chan error, 1)
	m := irrecoverable.NewMockSignalerContextWithCallback(t, context.Background(), func(err error) {
		thrown <- err
	})
	ctx := irrecoverable.WithSignalerContext(context.Background(), m)
	verify := func() {
		t.Helper()
		select {
		case err := <-thrown:
			require.Error(t, err)
		default:
			t.Fatal("expected irrecoverable.Throw to be called but it was not")
		}
	}
	return ctx, verify
}

// TestContractsBackend_GetContract tests all code paths for GetContract.
func TestContractsBackend_GetContract(t *testing.T) {
	t.Parallel()

	contractID := "A.0000000000000001.FungibleToken"
	deployment := makeContractDeployment(contractID, 42)

	t.Run("happy path returns deployment", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByContractID", contractID).Return(deployment, nil).Once()

		result, err := backend.GetContract(context.Background(), contractID, ContractDeploymentFilter{})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, contractID, result.ContractID)
		assert.Equal(t, uint64(42), result.BlockHeight)
	})

	t.Run("ErrNotFound returns codes.NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByContractID", contractID).Return(accessmodel.ContractDeployment{}, storage.ErrNotFound).Once()

		_, err := backend.GetContract(context.Background(), contractID, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByContractID", contractID).Return(accessmodel.ContractDeployment{}, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContract(context.Background(), contractID, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		unexpectedErr := fmt.Errorf("disk failure")
		mockStore.On("ByContractID", contractID).Return(accessmodel.ContractDeployment{}, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContract(signalerCtx, contractID, ContractDeploymentFilter{})
		require.Error(t, err)
		verifyThrown()
	})
}

// TestContractsBackend_GetContractDeployments tests all code paths for GetContractDeployments.
func TestContractsBackend_GetContractDeployments(t *testing.T) {
	t.Parallel()

	contractID := "A.0000000000000001.FungibleToken"

	t.Run("happy path returns page without next cursor", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(contractID, 50),
			makeContractDeployment(contractID, 30),
		}
		page := accessmodel.ContractDeploymentPage{Deployments: deployments}
		mockStore.On("DeploymentsByContractID", contractID, uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(page, nil).Once()

		result, err := backend.GetContractDeployments(context.Background(), contractID, 0, nil, ContractDeploymentFilter{})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Deployments, 2)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("has more results sets NextCursor", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentCursor{Height: 30, TxIndex: 0, EventIndex: 0}
		page := accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{makeContractDeployment(contractID, 50)},
			NextCursor:  cursor,
		}
		mockStore.On("DeploymentsByContractID", contractID, uint32(1),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(page, nil).Once()

		result, err := backend.GetContractDeployments(context.Background(), contractID, 1, nil, ContractDeploymentFilter{})
		require.NoError(t, err)
		require.NotNil(t, result.NextCursor)
		assert.Equal(t, uint64(30), result.NextCursor.Height)
	})

	t.Run("cursor forwarded to store", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentCursor{Height: 30, TxIndex: 1, EventIndex: 2}
		page := accessmodel.ContractDeploymentPage{}
		mockStore.On("DeploymentsByContractID", contractID, uint32(10), cursor, mocktestify.Anything).
			Return(page, nil).Once()

		_, err := backend.GetContractDeployments(context.Background(), contractID, 10, cursor, ContractDeploymentFilter{})
		require.NoError(t, err)
	})

	t.Run("limit exceeds max returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContractDeployments(context.Background(), contractID, DefaultConfig().MaxPageSize+1, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("DeploymentsByContractID", contractID, uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContractDeployments(context.Background(), contractID, 0, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		unexpectedErr := fmt.Errorf("disk failure")
		mockStore.On("DeploymentsByContractID", contractID, uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContractDeployments(signalerCtx, contractID, 0, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		verifyThrown()
	})
}

// TestContractsBackend_GetContracts tests all code paths for GetContracts.
func TestContractsBackend_GetContracts(t *testing.T) {
	t.Parallel()

	t.Run("happy path returns page", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment("A.0000000000000001.FungibleToken", 10),
			makeContractDeployment("A.0000000000000002.FlowToken", 11),
		}
		page := accessmodel.ContractDeploymentPage{Deployments: deployments}
		mockStore.On("All", uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(page, nil).Once()

		result, err := backend.GetContracts(context.Background(), 0, nil, ContractDeploymentFilter{})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Deployments, 2)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("has more results sets NextCursor", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentCursor{ContractID: "A.0000000000000001.FungibleToken"}
		page := accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{makeContractDeployment("A.0000000000000001.FungibleToken", 10)},
			NextCursor:  cursor,
		}
		mockStore.On("All", uint32(1), (*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(page, nil).Once()

		result, err := backend.GetContracts(context.Background(), 1, nil, ContractDeploymentFilter{})
		require.NoError(t, err)
		require.NotNil(t, result.NextCursor)
		assert.Equal(t, "A.0000000000000001.FungibleToken", result.NextCursor.ContractID)
	})

	t.Run("cursor forwarded to store", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentCursor{ContractID: "A.0000000000000001.FungibleToken"}
		mockStore.On("All", uint32(5), cursor, mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, nil).Once()

		_, err := backend.GetContracts(context.Background(), 5, cursor, ContractDeploymentFilter{})
		require.NoError(t, err)
	})

	t.Run("limit exceeds max returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContracts(context.Background(), DefaultConfig().MaxPageSize+1, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("All", uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContracts(context.Background(), 0, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		unexpectedErr := fmt.Errorf("disk failure")
		mockStore.On("All", uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContracts(signalerCtx, 0, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		verifyThrown()
	})
}

// TestContractsBackend_GetContractsByAddress tests all code paths for GetContractsByAddress.
func TestContractsBackend_GetContractsByAddress(t *testing.T) {
	t.Parallel()

	addr := unittest.RandomAddressFixture()

	t.Run("happy path returns page for address", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		contractID := fmt.Sprintf("A.%s.Foo", addr.Hex())
		page := accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{makeContractDeployment(contractID, 15)},
		}
		mockStore.On("ByAddress", addr, uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(page, nil).Once()

		result, err := backend.GetContractsByAddress(context.Background(), addr, 0, nil, ContractDeploymentFilter{})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Deployments, 1)
		assert.Equal(t, contractID, result.Deployments[0].ContractID)
	})

	t.Run("cursor forwarded to store", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentCursor{ContractID: fmt.Sprintf("A.%s.Foo", addr.Hex())}
		mockStore.On("ByAddress", addr, uint32(10), cursor, mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, nil).Once()

		_, err := backend.GetContractsByAddress(context.Background(), addr, 10, cursor, ContractDeploymentFilter{})
		require.NoError(t, err)
	})

	t.Run("limit exceeds max returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContractsByAddress(context.Background(), addr, DefaultConfig().MaxPageSize+1, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByAddress", addr, uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContractsByAddress(context.Background(), addr, 0, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("unexpected error triggers irrecoverable", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		unexpectedErr := fmt.Errorf("disk failure")
		mockStore.On("ByAddress", addr, uint32(DefaultConfig().DefaultPageSize),
			(*accessmodel.ContractDeploymentCursor)(nil), mocktestify.Anything).
			Return(accessmodel.ContractDeploymentPage{}, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContractsByAddress(signalerCtx, addr, 0, nil, ContractDeploymentFilter{})
		require.Error(t, err)
		verifyThrown()
	})
}

// TestContractDeploymentFilter tests the Filter() predicate builder.
func TestContractDeploymentFilter(t *testing.T) {
	t.Parallel()

	addr := unittest.RandomAddressFixture()
	fooID := fmt.Sprintf("A.%s.FungibleToken", addr.Hex())
	barID := fmt.Sprintf("A.%s.FlowToken", addr.Hex())

	foo := &accessmodel.ContractDeployment{ContractID: fooID, BlockHeight: 100}
	bar := &accessmodel.ContractDeployment{ContractID: barID, BlockHeight: 200}

	t.Run("empty filter passes all", func(t *testing.T) {
		t.Parallel()
		f := ContractDeploymentFilter{}
		filter := f.Filter()
		assert.True(t, filter(foo))
		assert.True(t, filter(bar))
	})

	t.Run("ContractName suffix match immediately passes, non-match falls through to block range", func(t *testing.T) {
		t.Parallel()
		// Filter{ContractName: "FungibleToken"} with no block range: name-matching contracts pass
		// via early return; non-matching contracts fall through to block range and also pass
		// (since no block range is set).
		f := ContractDeploymentFilter{ContractName: "FungibleToken"}
		filter := f.Filter()
		assert.True(t, filter(foo), "FungibleToken suffix matches → early true")
		assert.True(t, filter(bar), "FlowToken doesn't match name, falls through, no block range → true")
	})

	t.Run("ContractName with block range excludes non-matching out-of-range contract", func(t *testing.T) {
		t.Parallel()
		// bar is at height 200; set EndBlock=150 so bar fails the block range check.
		end := uint64(150)
		f := ContractDeploymentFilter{ContractName: "FungibleToken", EndBlock: &end}
		filter := f.Filter()
		assert.True(t, filter(foo), "FungibleToken matches name → early true, block range ignored")
		assert.False(t, filter(bar), "FlowToken fails name and height 200 > EndBlock 150 → false")
	})

	t.Run("StartBlock excludes deployments before the bound", func(t *testing.T) {
		t.Parallel()
		start := uint64(150)
		f := ContractDeploymentFilter{StartBlock: &start}
		filter := f.Filter()
		assert.False(t, filter(foo), "height 100 < StartBlock 150 should be excluded")
		assert.True(t, filter(bar), "height 200 >= StartBlock 150 should be included")
	})

	t.Run("EndBlock excludes deployments after the bound", func(t *testing.T) {
		t.Parallel()
		end := uint64(150)
		f := ContractDeploymentFilter{EndBlock: &end}
		filter := f.Filter()
		assert.True(t, filter(foo), "height 100 <= EndBlock 150 should be included")
		assert.False(t, filter(bar), "height 200 > EndBlock 150 should be excluded")
	})

	t.Run("StartBlock and EndBlock window", func(t *testing.T) {
		t.Parallel()
		start := uint64(50)
		end := uint64(150)
		f := ContractDeploymentFilter{StartBlock: &start, EndBlock: &end}
		filter := f.Filter()
		assert.True(t, filter(foo), "height 100 within [50, 150] should be included")
		assert.False(t, filter(bar), "height 200 outside [50, 150] should be excluded")
	})
}
