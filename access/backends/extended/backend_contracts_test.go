package extended

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// makeContractDeployment builds a minimal ContractDeployment for use in backend tests.
// contractID must have the format "A.{16hex}.{name}".
func makeContractDeployment(tb testing.TB, contractID string, height uint64) accessmodel.ContractDeployment {
	tb.Helper()
	addr, name, err := accessmodel.ParseContractID(contractID)
	require.NoError(tb, err)
	return accessmodel.ContractDeployment{
		Address:       addr,
		ContractName:  name,
		BlockHeight:   height,
		TransactionID: unittest.IdentifierFixture(),
		Code:          []byte("access(all) contract Foo {}"),
		CodeHash:      make([]byte, 32),
	}
}

// testIterEntry is a storage.IteratorEntry used by both deployment-history and contracts iterators.
// The cursor is pre-computed at construction time via newDeploymentHistoryEntry or newContractsEntry.
type testIterEntry struct {
	d      accessmodel.ContractDeployment
	cursor accessmodel.ContractDeploymentsCursor
}

func (e testIterEntry) Cursor() accessmodel.ContractDeploymentsCursor { return e.cursor }
func (e testIterEntry) Value() (accessmodel.ContractDeployment, error) { return e.d, nil }

// newDeploymentHistoryEntry builds a testIterEntry whose cursor is keyed by
// BlockHeight/TransactionIndex/EventIndex (used by DeploymentsByContract).
func newDeploymentHistoryEntry(d accessmodel.ContractDeployment) testIterEntry {
	return testIterEntry{d: d, cursor: accessmodel.ContractDeploymentsCursor{
		BlockHeight:      d.BlockHeight,
		TransactionIndex: d.TransactionIndex,
		EventIndex:       d.EventIndex,
	}}
}

// newContractsEntry builds a testIterEntry whose cursor is keyed by Address/ContractName
// (used by All and ByAddress).
func newContractsEntry(d accessmodel.ContractDeployment) testIterEntry {
	return testIterEntry{d: d, cursor: accessmodel.ContractDeploymentsCursor{
		Address:      d.Address,
		ContractName: d.ContractName,
	}}
}

// makeContractDeploymentIter builds a storage.ContractDeploymentIterator from a slice of deployments.
// Used for DeploymentsByContractID (cursor type: ContractDeploymentsCursor).
func makeContractDeploymentIter(deployments []accessmodel.ContractDeployment) storage.ContractDeploymentIterator {
	return func(yield func(storage.IteratorEntry[accessmodel.ContractDeployment, accessmodel.ContractDeploymentsCursor], error) bool) {
		for _, d := range deployments {
			if !yield(newDeploymentHistoryEntry(d), nil) {
				return
			}
		}
	}
}

// makeContractsIter builds a storage.ContractDeploymentIterator from a slice of deployments.
// Used for All and ByAddress (cursor type: ContractsCursor).
func makeContractsIter(deployments []accessmodel.ContractDeployment) storage.ContractDeploymentIterator {
	return func(yield func(storage.IteratorEntry[accessmodel.ContractDeployment, accessmodel.ContractDeploymentsCursor], error) bool) {
		for _, d := range deployments {
			if !yield(newContractsEntry(d), nil) {
				return
			}
		}
	}
}

// makeIterWithError returns an iterator that immediately yields the given error.
// Used to test error handling in CollectResults.
func makeIterWithError(err error) storage.ContractDeploymentIterator {
	return func(yield func(storage.IteratorEntry[accessmodel.ContractDeployment, accessmodel.ContractDeploymentsCursor], error) bool) {
		yield(nil, err)
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
	contractAddr := flow.HexToAddress("0000000000000001")
	contractName := "FungibleToken"
	deployment := makeContractDeployment(t, contractID, 42)

	t.Run("happy path returns deployment", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByContract", contractAddr, contractName).Return(deployment, nil).Once()

		result, err := backend.GetContract(context.Background(), contractID, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, contractID, accessmodel.ContractID(result.Address, result.ContractName))
		assert.Equal(t, uint64(42), result.BlockHeight)
	})

	t.Run("ErrNotFound returns codes.NotFound", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByContract", contractAddr, contractName).Return(accessmodel.ContractDeployment{}, storage.ErrNotFound).Once()

		_, err := backend.GetContract(context.Background(), contractID, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByContract", contractAddr, contractName).Return(accessmodel.ContractDeployment{}, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContract(context.Background(), contractID, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
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
		mockStore.On("ByContract", contractAddr, contractName).Return(accessmodel.ContractDeployment{}, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContract(signalerCtx, contractID, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("invalid contract ID returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContract(context.Background(), "notavalidid", ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// TestContractsBackend_GetContractDeployments tests all code paths for GetContractDeployments.
func TestContractsBackend_GetContractDeployments(t *testing.T) {
	t.Parallel()

	contractID := "A.0000000000000001.FungibleToken"
	contractAddr := flow.HexToAddress("0000000000000001")
	contractName := "FungibleToken"

	t.Run("happy path returns page without next cursor", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(t, contractID, 50),
			makeContractDeployment(t, contractID, 30),
		}
		mockStore.On("DeploymentsByContract", contractAddr, contractName, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractDeploymentIter(deployments), nil).Once()

		result, err := backend.GetContractDeployments(context.Background(), contractID, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Deployments, 2)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("has more results sets NextCursor", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		// Provide 2 items for limit=1: first is returned, second becomes the cursor.
		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(t, contractID, 50),
			makeContractDeployment(t, contractID, 30), // cursor item (first of next page)
		}
		mockStore.On("DeploymentsByContract", contractAddr, contractName, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractDeploymentIter(deployments), nil).Once()

		result, err := backend.GetContractDeployments(context.Background(), contractID, 1, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.NotNil(t, result.NextCursor)
		assert.Equal(t, uint64(30), result.NextCursor.BlockHeight)
	})

	t.Run("cursor forwarded to store", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentsCursor{BlockHeight: 30, TransactionIndex: 1, EventIndex: 2}
		mockStore.On("DeploymentsByContract", contractAddr, contractName, cursor).
			Return(makeContractDeploymentIter(nil), nil).Once()

		_, err := backend.GetContractDeployments(context.Background(), contractID, 10, cursor, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
	})

	t.Run("limit exceeds max returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContractDeployments(context.Background(), contractID, DefaultConfig().MaxPageSize+1, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("DeploymentsByContract", contractAddr, contractName, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(nil, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContractDeployments(context.Background(), contractID, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
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
		mockStore.On("DeploymentsByContract", contractAddr, contractName, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(nil, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContractDeployments(signalerCtx, contractID, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("iterator error triggers irrecoverable", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		iterErr := fmt.Errorf("storage read failure")
		mockStore.On("DeploymentsByContract", contractAddr, contractName, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeIterWithError(iterErr), nil).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContractDeployments(signalerCtx, contractID, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("filter excludes out-of-range deployments", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(t, contractID, 50),
			makeContractDeployment(t, contractID, 30),
			makeContractDeployment(t, contractID, 10),
		}
		mockStore.On("DeploymentsByContract", contractAddr, contractName, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractDeploymentIter(deployments), nil).Once()

		start, end := uint64(20), uint64(40)
		result, err := backend.GetContractDeployments(context.Background(), contractID, 0, nil,
			ContractDeploymentFilter{StartBlock: &start, EndBlock: &end},
			ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.Len(t, result.Deployments, 1)
		assert.Equal(t, uint64(30), result.Deployments[0].BlockHeight)
	})

	t.Run("invalid contract ID returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContractDeployments(context.Background(), "notavalidid", 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
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
			makeContractDeployment(t, "A.0000000000000001.FungibleToken", 10),
			makeContractDeployment(t, "A.0000000000000002.FlowToken", 11),
		}
		mockStore.On("All", (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractsIter(deployments), nil).Once()

		result, err := backend.GetContracts(context.Background(), 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.Deployments, 2)
		assert.Nil(t, result.NextCursor)
	})

	t.Run("has more results sets NextCursor", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		// Provide 2 items for limit=1: first is returned, second becomes the cursor.
		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(t, "A.0000000000000001.FungibleToken", 10),
			makeContractDeployment(t, "A.0000000000000002.FlowToken", 11), // cursor item
		}
		mockStore.On("All", (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractsIter(deployments), nil).Once()

		result, err := backend.GetContracts(context.Background(), 1, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.NotNil(t, result.NextCursor)
		assert.Equal(t, flow.HexToAddress("0000000000000002"), result.NextCursor.Address)
		assert.Equal(t, "FlowToken", result.NextCursor.ContractName)
	})

	t.Run("cursor forwarded to store", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentsCursor{Address: flow.HexToAddress("0000000000000001"), ContractName: "FungibleToken"}
		mockStore.On("All", cursor).
			Return(makeContractsIter(nil), nil).Once()

		_, err := backend.GetContracts(context.Background(), 5, cursor, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
	})

	t.Run("limit exceeds max returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContracts(context.Background(), DefaultConfig().MaxPageSize+1, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("All", (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(nil, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContracts(context.Background(), 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
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
		mockStore.On("All", (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(nil, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContracts(signalerCtx, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("iterator error triggers irrecoverable", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		iterErr := fmt.Errorf("storage read failure")
		mockStore.On("All", (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeIterWithError(iterErr), nil).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContracts(signalerCtx, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("filter by contract name excludes non-matching contracts", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(t, "A.0000000000000001.FungibleToken", 10),
			makeContractDeployment(t, "A.0000000000000002.FlowToken", 11),
		}
		mockStore.On("All", (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractsIter(deployments), nil).Once()

		result, err := backend.GetContracts(context.Background(), 0, nil,
			ContractDeploymentFilter{ContractName: "FungibleToken"},
			ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.Len(t, result.Deployments, 1)
		assert.Equal(t, "FungibleToken", result.Deployments[0].ContractName)
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
		deployments := []accessmodel.ContractDeployment{makeContractDeployment(t, contractID, 15)}
		mockStore.On("ByAddress", addr, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractsIter(deployments), nil).Once()

		result, err := backend.GetContractsByAddress(context.Background(), addr, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Deployments, 1)
		assert.Equal(t, contractID, accessmodel.ContractID(result.Deployments[0].Address, result.Deployments[0].ContractName))
	})

	t.Run("has more results sets NextCursor", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		contractID1 := fmt.Sprintf("A.%s.Bar", addr.Hex())
		contractID2 := fmt.Sprintf("A.%s.Foo", addr.Hex())
		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(t, contractID1, 10),
			makeContractDeployment(t, contractID2, 11), // cursor item (first of next page)
		}
		mockStore.On("ByAddress", addr, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractsIter(deployments), nil).Once()

		result, err := backend.GetContractsByAddress(context.Background(), addr, 1, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.NotNil(t, result.NextCursor)
		assert.Equal(t, addr, result.NextCursor.Address)
		assert.Equal(t, "Foo", result.NextCursor.ContractName)
	})

	t.Run("cursor forwarded to store", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		cursor := &accessmodel.ContractDeploymentsCursor{Address: addr, ContractName: "Foo"}
		mockStore.On("ByAddress", addr, cursor).
			Return(makeContractsIter(nil), nil).Once()

		_, err := backend.GetContractsByAddress(context.Background(), addr, 10, cursor, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
	})

	t.Run("limit exceeds max returns codes.InvalidArgument", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		_, err := backend.GetContractsByAddress(context.Background(), addr, DefaultConfig().MaxPageSize+1, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("ErrNotBootstrapped returns codes.FailedPrecondition", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		mockStore.On("ByAddress", addr, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(nil, storage.ErrNotBootstrapped).Once()

		_, err := backend.GetContractsByAddress(context.Background(), addr, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
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
		mockStore.On("ByAddress", addr, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(nil, unexpectedErr).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContractsByAddress(signalerCtx, addr, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("iterator error triggers irrecoverable", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		iterErr := fmt.Errorf("storage read failure")
		mockStore.On("ByAddress", addr, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeIterWithError(iterErr), nil).Once()

		signalerCtx, verifyThrown := contractSignalerCtxExpectingThrow(t)
		_, err := backend.GetContractsByAddress(signalerCtx, addr, 0, nil, ContractDeploymentFilter{}, ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.Error(t, err)
		verifyThrown()
	})

	t.Run("filter excludes out-of-range contracts", func(t *testing.T) {
		t.Parallel()
		mockStore := storagemock.NewContractDeploymentsIndexReader(t)
		backend := NewContractsBackend(unittest.Logger(), &backendBase{config: DefaultConfig()}, mockStore)

		contractID1 := fmt.Sprintf("A.%s.Bar", addr.Hex())
		contractID2 := fmt.Sprintf("A.%s.Foo", addr.Hex())
		deployments := []accessmodel.ContractDeployment{
			makeContractDeployment(t, contractID1, 10),
			makeContractDeployment(t, contractID2, 50),
		}
		mockStore.On("ByAddress", addr, (*accessmodel.ContractDeploymentsCursor)(nil)).
			Return(makeContractsIter(deployments), nil).Once()

		end := uint64(30)
		result, err := backend.GetContractsByAddress(context.Background(), addr, 0, nil,
			ContractDeploymentFilter{EndBlock: &end},
			ContractDeploymentExpandOptions{}, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)
		require.Len(t, result.Deployments, 1)
		assert.Equal(t, "Bar", result.Deployments[0].ContractName)
	})
}

// TestContractDeploymentFilter tests the Filter() predicate builder.
func TestContractDeploymentFilter(t *testing.T) {
	t.Parallel()

	addr := unittest.RandomAddressFixture()

	foo := &accessmodel.ContractDeployment{Address: addr, ContractName: "FungibleToken", BlockHeight: 100}
	bar := &accessmodel.ContractDeployment{Address: addr, ContractName: "FlowToken", BlockHeight: 200}

	t.Run("empty filter always filters deleted contracts", func(t *testing.T) {
		t.Parallel()
		f := ContractDeploymentFilter{}
		filter := f.Filter()
		require.NotNil(t, filter)
		assert.True(t, filter(foo), "non-deleted contract passes empty filter")
		deleted := &accessmodel.ContractDeployment{Address: addr, ContractName: "FungibleToken", BlockHeight: 100, IsDeleted: true}
		assert.False(t, filter(deleted), "deleted contract is filtered out")
	})

	t.Run("ContractName exact match filters non-matching names", func(t *testing.T) {
		t.Parallel()
		// Filter{ContractName: "FungibleToken"}: matching name passes, non-matching name fails.
		f := ContractDeploymentFilter{ContractName: "FungibleToken"}
		filter := f.Filter()
		assert.True(t, filter(foo), "FungibleToken matches → true")
		assert.False(t, filter(bar), "FlowToken doesn't match FungibleToken → false")
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

	t.Run("StartBlock boundary is inclusive", func(t *testing.T) {
		t.Parallel()
		start := uint64(100) // exactly foo's height
		f := ContractDeploymentFilter{StartBlock: &start}
		filter := f.Filter()
		assert.True(t, filter(foo), "height 100 == StartBlock 100 should be included")
		assert.True(t, filter(bar), "height 200 > StartBlock 100 should be included")
	})

	t.Run("EndBlock boundary is inclusive", func(t *testing.T) {
		t.Parallel()
		end := uint64(100) // exactly foo's height
		f := ContractDeploymentFilter{EndBlock: &end}
		filter := f.Filter()
		assert.True(t, filter(foo), "height 100 == EndBlock 100 should be included")
		assert.False(t, filter(bar), "height 200 > EndBlock 100 should be excluded")
	})
}
