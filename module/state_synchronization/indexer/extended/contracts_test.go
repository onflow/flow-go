package extended_test

import (
	"crypto/sha3"
	"fmt"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	executionmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"

	. "github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
)

const contractsTestHeight = uint64(100)

// ===== Happy-path tests =====

// TestContractsIndexer_NoEvents verifies that indexing a block with no contract events
// stores no deployments and advances the latest indexed height.
func TestContractsIndexer_NoEvents(t *testing.T) {
	t.Parallel()

	indexer, store, lm, db := newContractsIndexerForTest(t, flow.Testnet, contractsTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))

	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{},
	})

	latest, err := store.LatestIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, contractsTestHeight, latest)

	page, err := store.All(1, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, page.Deployments)
}

// TestContractsIndexer_NextHeight_NotBootstrapped verifies that NextHeight returns the
// configured first height before any blocks have been indexed.
func TestContractsIndexer_NextHeight_NotBootstrapped(t *testing.T) {
	t.Parallel()

	indexer, _, _, _ := newContractsIndexerForTest(t, flow.Testnet, contractsTestHeight)

	height, err := indexer.NextHeight()
	require.NoError(t, err)
	assert.Equal(t, contractsTestHeight, height)
}

// TestContractsIndexer_ContractAdded verifies that an AccountContractAdded event creates
// a deployment entry with all fields correctly stored.
func TestContractsIndexer_ContractAdded(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "MyContract"
	code := []byte("access(all) contract MyContract {}")
	codeHash := cadenceCodeHash(code)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsTestHeight, scriptExecutor)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))
	deployingTxID := unittest.IdentifierFixture()

	// Script executor returns the account with the contract code.
	account := &flow.Account{
		Address:   address,
		Contracts: map[string][]byte{contractName: code},
	}
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, contractsTestHeight).
		Return(account, nil).Once()

	event := makeAccountContractAddedEvent(t, address, contractName, codeHash, 0, 0)
	event.TransactionID = deployingTxID

	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{event},
	})

	contractID := fmt.Sprintf("A.%s.%s", address.Hex(), contractName)
	deployment, err := store.ByContractID(contractID)
	require.NoError(t, err)

	assert.Equal(t, contractID, deployment.ContractID)
	assert.Equal(t, address, deployment.Address)
	assert.Equal(t, contractsTestHeight, deployment.BlockHeight)
	assert.Equal(t, deployingTxID, deployment.TransactionID)
	assert.Equal(t, uint32(0), deployment.TxIndex)
	assert.Equal(t, uint32(0), deployment.EventIndex)
	assert.Equal(t, code, deployment.Code)
	assert.Equal(t, codeHash, deployment.CodeHash)
}

// TestContractsIndexer_ContractUpdated verifies that an AccountContractUpdated event at a
// subsequent block creates a new deployment entry for the same contract.
func TestContractsIndexer_ContractUpdated(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "MyContract"
	codeV1 := []byte("access(all) contract MyContract { let x: Int; init() { self.x = 1 } }")
	codeV2 := []byte("access(all) contract MyContract { let x: Int; init() { self.x = 2 } }")
	codeHashV1 := cadenceCodeHash(codeV1)
	codeHashV2 := cadenceCodeHash(codeV2)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsTestHeight, scriptExecutor)

	// Height 1: AccountContractAdded
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))
	accountV1 := &flow.Account{Address: address, Contracts: map[string][]byte{contractName: codeV1}}
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, contractsTestHeight).
		Return(accountV1, nil).Once()

	addEvent := makeAccountContractAddedEvent(t, address, contractName, codeHashV1, 0, 0)
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header1, Events: []flow.Event{addEvent}})

	// Height 2: AccountContractUpdated
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight+1))
	accountV2 := &flow.Account{Address: address, Contracts: map[string][]byte{contractName: codeV2}}
	updateTxID := unittest.IdentifierFixture()
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, contractsTestHeight+1).
		Return(accountV2, nil).Once()

	updateEvent := makeAccountContractUpdatedEvent(t, address, contractName, codeHashV2, 1, 0)
	updateEvent.TransactionID = updateTxID
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header2, Events: []flow.Event{updateEvent}})

	contractID := fmt.Sprintf("A.%s.%s", address.Hex(), contractName)

	// ByContractID returns the most recent deployment (height 2).
	latest, err := store.ByContractID(contractID)
	require.NoError(t, err)
	assert.Equal(t, contractsTestHeight+1, latest.BlockHeight)
	assert.Equal(t, codeV2, latest.Code)
	assert.Equal(t, updateTxID, latest.TransactionID)

	// DeploymentsByContractID returns both deployments, most recent first.
	page, err := store.DeploymentsByContractID(contractID, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, page.Deployments, 2)
	assert.Equal(t, contractsTestHeight+1, page.Deployments[0].BlockHeight)
	assert.Equal(t, contractsTestHeight, page.Deployments[1].BlockHeight)
}

// TestContractsIndexer_MultipleContractsInOneBlock verifies that multiple AccountContractAdded
// events in a single block each create a separate deployment entry.
func TestContractsIndexer_MultipleContractsInOneBlock(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()
	code1 := []byte("access(all) contract Alpha {}")
	code2 := []byte("access(all) contract Beta {}")
	hash1 := cadenceCodeHash(code1)
	hash2 := cadenceCodeHash(code2)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsTestHeight, scriptExecutor)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))

	// Two different accounts, each with one contract.
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, addr1, contractsTestHeight).
		Return(&flow.Account{Address: addr1, Contracts: map[string][]byte{"Alpha": code1}}, nil).Once()
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, addr2, contractsTestHeight).
		Return(&flow.Account{Address: addr2, Contracts: map[string][]byte{"Beta": code2}}, nil).Once()

	event1 := makeAccountContractAddedEvent(t, addr1, "Alpha", hash1, 0, 0)
	event2 := makeAccountContractAddedEvent(t, addr2, "Beta", hash2, 1, 0)

	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{event1, event2},
	})

	id1 := fmt.Sprintf("A.%s.Alpha", addr1.Hex())
	id2 := fmt.Sprintf("A.%s.Beta", addr2.Hex())

	d1, err := store.ByContractID(id1)
	require.NoError(t, err)
	assert.Equal(t, code1, d1.Code)

	d2, err := store.ByContractID(id2)
	require.NoError(t, err)
	assert.Equal(t, code2, d2.Code)

	// All() returns both contracts (latest deployment of each).
	page, err := store.All(10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Deployments, 2)
}

// TestContractsIndexer_SameAccountCached verifies that when one block deploys two contracts
// to the same account, GetAccountAtBlockHeight is called only once (result is cached).
func TestContractsIndexer_SameAccountCached(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	codeA := []byte("access(all) contract Alpha {}")
	codeB := []byte("access(all) contract Beta {}")
	hashA := cadenceCodeHash(codeA)
	hashB := cadenceCodeHash(codeB)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsTestHeight, scriptExecutor)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))

	// GetAccountAtBlockHeight must be called exactly once (caching).
	account := &flow.Account{
		Address:   address,
		Contracts: map[string][]byte{"Alpha": codeA, "Beta": codeB},
	}
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, contractsTestHeight).
		Return(account, nil).Once()

	eventA := makeAccountContractAddedEvent(t, address, "Alpha", hashA, 0, 0)
	eventB := makeAccountContractAddedEvent(t, address, "Beta", hashB, 0, 1)

	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{eventA, eventB},
	})

	page, err := store.ByAddress(address, 10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Deployments, 2)
}

// TestContractsIndexer_ByAddress verifies that ByAddress returns only contracts for the
// requested account, across multiple indexed blocks.
func TestContractsIndexer_ByAddress(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()
	code1 := []byte("access(all) contract Foo {}")
	code2 := []byte("access(all) contract Bar {}")

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsTestHeight, scriptExecutor)

	// Block 1: addr1 deploys Foo.
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, addr1, contractsTestHeight).
		Return(&flow.Account{Address: addr1, Contracts: map[string][]byte{"Foo": code1}}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{makeAccountContractAddedEvent(t, addr1, "Foo", cadenceCodeHash(code1), 0, 0)},
	})

	// Block 2: addr2 deploys Bar.
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight+1))
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, addr2, contractsTestHeight+1).
		Return(&flow.Account{Address: addr2, Contracts: map[string][]byte{"Bar": code2}}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header2,
		Events: []flow.Event{makeAccountContractAddedEvent(t, addr2, "Bar", cadenceCodeHash(code2), 0, 0)},
	})

	// ByAddress(addr1) returns only Foo.
	page1, err := store.ByAddress(addr1, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, page1.Deployments, 1)
	assert.Equal(t, fmt.Sprintf("A.%s.Foo", addr1.Hex()), page1.Deployments[0].ContractID)

	// ByAddress(addr2) returns only Bar.
	page2, err := store.ByAddress(addr2, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, page2.Deployments, 1)
	assert.Equal(t, fmt.Sprintf("A.%s.Bar", addr2.Hex()), page2.Deployments[0].ContractID)
}

// ===== Backfill tests =====

// TestContractsIndexer_Backfill verifies the full backfill scenario: the index bootstraps
// at firstHeight (no events), then processes subsequent blocks with contract events via
// backfill, resulting in correct deployment records.
func TestContractsIndexer_Backfill(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "BackfillContract"
	codeV1 := []byte("access(all) contract BackfillContract { let v: Int; init() { self.v = 1 } }")
	codeV2 := []byte("access(all) contract BackfillContract { let v: Int; init() { self.v = 2 } }")
	hashV1 := cadenceCodeHash(codeV1)
	hashV2 := cadenceCodeHash(codeV2)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	firstHeight := contractsTestHeight
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, firstHeight, scriptExecutor)

	// Height 100 (firstHeight): bootstrap with no events.
	header100 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight))
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header100, Events: []flow.Event{}})

	first, err := store.FirstIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, firstHeight, first)

	// Height 101: contract added — this represents a backfilled block.
	header101 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight+1))
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, firstHeight+1).
		Return(&flow.Account{Address: address, Contracts: map[string][]byte{contractName: codeV1}}, nil).Once()

	addTxID := unittest.IdentifierFixture()
	addEvent := makeAccountContractAddedEvent(t, address, contractName, hashV1, 2, 0)
	addEvent.TransactionID = addTxID
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header101, Events: []flow.Event{addEvent}})

	// Height 102: contract updated.
	header102 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight+2))
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, firstHeight+2).
		Return(&flow.Account{Address: address, Contracts: map[string][]byte{contractName: codeV2}}, nil).Once()

	updateTxID := unittest.IdentifierFixture()
	updateEvent := makeAccountContractUpdatedEvent(t, address, contractName, hashV2, 0, 0)
	updateEvent.TransactionID = updateTxID
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header102, Events: []flow.Event{updateEvent}})

	latest, err := store.LatestIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, firstHeight+2, latest)

	contractID := fmt.Sprintf("A.%s.%s", address.Hex(), contractName)

	// ByContractID returns the latest (height 102) deployment.
	d, err := store.ByContractID(contractID)
	require.NoError(t, err)
	assert.Equal(t, firstHeight+2, d.BlockHeight)
	assert.Equal(t, codeV2, d.Code)
	assert.Equal(t, updateTxID, d.TransactionID)

	// DeploymentsByContractID returns both deployments ordered most recent first.
	page, err := store.DeploymentsByContractID(contractID, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, page.Deployments, 2)
	assert.Equal(t, firstHeight+2, page.Deployments[0].BlockHeight)
	assert.Equal(t, firstHeight+1, page.Deployments[1].BlockHeight)
	assert.Equal(t, addTxID, page.Deployments[1].TransactionID)
}

// TestContractsIndexer_BackfillMultipleBlocks verifies that starting the index at a root
// height and backfilling several subsequent blocks processes each block in order and accumulates
// deployment records correctly.
func TestContractsIndexer_BackfillMultipleBlocks(t *testing.T) {
	t.Parallel()

	// Three distinct contracts deployed across three blocks.
	addrs := make([]flow.Address, 3)
	codes := make([][]byte, 3)
	hashes := make([][]byte, 3)
	for i := range addrs {
		addrs[i] = unittest.RandomAddressFixture()
		codes[i] = fmt.Appendf(nil, "access(all) contract C%d {}", i)
		hashes[i] = cadenceCodeHash(codes[i])
	}

	scriptExecutor := executionmock.NewScriptExecutor(t)
	firstHeight := contractsTestHeight
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, firstHeight, scriptExecutor)

	// Root block: no events.
	root := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight))
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: root, Events: []flow.Event{}})

	// Index three blocks, one contract added per block.
	for i := range 3 {
		h := firstHeight + uint64(i+1)
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(h))
		scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, addrs[i], h).
			Return(&flow.Account{Address: addrs[i], Contracts: map[string][]byte{fmt.Sprintf("C%d", i): codes[i]}}, nil).Once()
		event := makeAccountContractAddedEvent(t, addrs[i], fmt.Sprintf("C%d", i), hashes[i], 0, 0)
		indexContractsBlock(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{event}})
	}

	latest, err := store.LatestIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, firstHeight+3, latest)

	// All() returns all three contracts.
	page, err := store.All(10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Deployments, 3)
}

// ===== Error and edge-case tests =====

// TestContractsIndexer_AlreadyIndexed verifies that attempting to index the same height
// twice returns ErrAlreadyIndexed.
func TestContractsIndexer_AlreadyIndexed(t *testing.T) {
	t.Parallel()

	indexer, _, lm, db := newContractsIndexerForTest(t, flow.Testnet, contractsTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))

	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{}})

	err := indexContractsBlockExpectError(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{}})
	require.ErrorIs(t, err, ErrAlreadyIndexed)
}

// TestContractsIndexer_ContractRemoved verifies that an AccountContractRemoved event
// returns an error (contract removal is not supported).
func TestContractsIndexer_ContractRemoved(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	indexer, _, lm, db := newContractsIndexerForTest(t, flow.Testnet, contractsTestHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))

	removedEvent := makeAccountContractRemovedEvent(t, address, "SomeContract", make([]byte, 32), 0, 0)

	err := indexContractsBlockExpectError(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{removedEvent},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// TestContractsIndexer_CodeHashMismatch verifies that when the code retrieved via the
// script executor does not match the hash in the event, the deployment is stored without
// code (best-effort — no crash), but the code hash from the event is always stored.
func TestContractsIndexer_CodeHashMismatch(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "Mismatch"
	realCode := []byte("access(all) contract Mismatch {}")
	wrongCode := []byte("access(all) contract Different {}")
	// Event carries hash of realCode, but account returns wrongCode.
	eventHash := cadenceCodeHash(realCode)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsTestHeight, scriptExecutor)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, contractsTestHeight).
		Return(&flow.Account{Address: address, Contracts: map[string][]byte{contractName: wrongCode}}, nil).Once()

	event := makeAccountContractAddedEvent(t, address, contractName, eventHash, 0, 0)

	// Should NOT fail — code hash mismatch is treated as best-effort warning.
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{event},
	})

	contractID := fmt.Sprintf("A.%s.%s", address.Hex(), contractName)
	deployment, err := store.ByContractID(contractID)
	require.NoError(t, err)
	assert.Equal(t, eventHash, deployment.CodeHash, "code hash from event should always be stored")
	assert.Nil(t, deployment.Code, "code should be nil when hash mismatch occurs")
}

// TestContractsIndexer_ScriptExecutorError verifies that an error from GetAccountAtBlockHeight
// is treated as best-effort: the deployment is stored without code (Code = nil) and no error
// is returned from IndexBlockData. The code hash from the event is always stored.
func TestContractsIndexer_ScriptExecutorError(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	scriptErr := fmt.Errorf("storage unavailable")

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsTestHeight, scriptExecutor)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsTestHeight))
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, address, contractsTestHeight).
		Return(nil, scriptErr).Once()

	codeHash := cadenceCodeHash([]byte("access(all) contract MyContract {}"))
	event := makeAccountContractAddedEvent(t, address, "MyContract", codeHash, 0, 0)

	// Should NOT fail — code retrieval is best-effort.
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{event},
	})

	contractID := fmt.Sprintf("A.%s.MyContract", address.Hex())
	deployment, err := store.ByContractID(contractID)
	require.NoError(t, err)
	assert.Equal(t, contractID, deployment.ContractID)
	assert.Equal(t, codeHash, deployment.CodeHash, "code hash from event should always be stored")
	assert.Nil(t, deployment.Code, "code should be nil when retrieval fails")
}

// TestContractsIndexer_NextHeight_MockErrors verifies error propagation from the store.
func TestContractsIndexer_NextHeight_MockErrors(t *testing.T) {
	t.Parallel()

	t.Run("unexpected error from LatestIndexedHeight propagates", func(t *testing.T) {
		t.Parallel()

		mockStore := storagemock.NewContractDeploymentsIndexBootstrapper(t)
		unexpectedErr := fmt.Errorf("disk I/O failure")
		mockStore.On("LatestIndexedHeight").Return(uint64(0), unexpectedErr)

		indexer := NewContracts(unittest.Logger(), flow.Testnet.Chain(), mockStore, nil, &metrics.NoopCollector{})

		_, err := indexer.NextHeight()
		require.Error(t, err)
		require.ErrorIs(t, err, unexpectedErr)
	})

	t.Run("inconsistent state: not bootstrapped but initialized", func(t *testing.T) {
		t.Parallel()

		mockStore := storagemock.NewContractDeploymentsIndexBootstrapper(t)
		mockStore.On("LatestIndexedHeight").Return(uint64(0), storage.ErrNotBootstrapped)
		mockStore.On("UninitializedFirstHeight").Return(uint64(42), true)

		indexer := NewContracts(unittest.Logger(), flow.Testnet.Chain(), mockStore, nil, &metrics.NoopCollector{})

		_, err := indexer.NextHeight()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "but index is initialized")
	})

	t.Run("store error from Store propagates", func(t *testing.T) {
		t.Parallel()

		const testHeight = uint64(100)
		mockStore := storagemock.NewContractDeploymentsIndexBootstrapper(t)
		// Contracts.IndexBlockData does not call NextHeight/LatestIndexedHeight before storing;
		// it delegates height validation to the store's Store implementation.
		storeErr := fmt.Errorf("unexpected storage error")
		mockStore.On("Store", mock.Anything, mock.Anything, testHeight, mock.Anything).Return(storeErr)

		lm := storage.NewTestingLockManager()
		indexer := NewContracts(unittest.Logger(), flow.Testnet.Chain(), mockStore, nil, &metrics.NoopCollector{})
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))

		err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
			return indexer.IndexBlockData(lctx, BlockData{Header: header, Events: []flow.Event{}}, nil)
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storeErr)
	})
}

// ===== Test Setup Helpers =====

// newContractsIndexerForTest creates a Contracts indexer backed by a real pebble DB with
// no script executor (suitable for tests without any contract events).
func newContractsIndexerForTest(
	t *testing.T,
	chainID flow.ChainID,
	firstHeight uint64,
) (*Contracts, storage.ContractDeploymentsIndexBootstrapper, storage.LockManager, storage.DB) {
	return newContractsIndexerWithScriptExecutor(t, chainID, firstHeight, nil)
}

// newContractsIndexerWithScriptExecutor creates a Contracts indexer backed by a real pebble DB
// with the given script executor.
func newContractsIndexerWithScriptExecutor(
	t *testing.T,
	chainID flow.ChainID,
	firstHeight uint64,
	scriptExecutor *executionmock.ScriptExecutor,
) (*Contracts, storage.ContractDeploymentsIndexBootstrapper, storage.LockManager, storage.DB) {
	pdb, dbDir := unittest.TempPebbleDB(t)
	db := pebbleimpl.ToDB(pdb)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})

	lm := storage.NewTestingLockManager()
	store, err := indexes.NewContractDeploymentsBootstrapper(db, firstHeight)
	require.NoError(t, err)

	indexer := NewContracts(unittest.Logger(), chainID.Chain(), store, scriptExecutor, &metrics.NoopCollector{})
	return indexer, store, lm, db
}

// indexContractsBlock runs IndexBlockData with proper locking and batch commit, requiring no error.
func indexContractsBlock(
	t *testing.T,
	indexer *Contracts,
	lm storage.LockManager,
	db storage.DB,
	data BlockData,
) {
	err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return indexer.IndexBlockData(lctx, data, rw)
		})
	})
	require.NoError(t, err)
}

// indexContractsBlockExpectError runs IndexBlockData and returns the error without failing.
func indexContractsBlockExpectError(
	t *testing.T,
	indexer *Contracts,
	lm storage.LockManager,
	db storage.DB,
	data BlockData,
) error {
	return unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return indexer.IndexBlockData(lctx, data, rw)
		})
	})
}

// ===== Event Creation Helpers =====

// makeAccountContractAddedEvent builds a CCF-encoded flow.AccountContractAdded event.
func makeAccountContractAddedEvent(
	t *testing.T,
	address flow.Address,
	contractName string,
	codeHash []byte,
	txIndex uint32,
	eventIndex uint32,
) flow.Event {
	t.Helper()

	eventType := cadence.NewEventType(
		nil,
		"flow.AccountContractAdded",
		[]cadence.Field{
			{Identifier: "address", Type: cadence.AddressType},
			{Identifier: "codeHash", Type: cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)},
			{Identifier: "contract", Type: cadence.StringType},
		},
		nil,
	)

	hashValues := make([]cadence.Value, 32)
	for i, b := range codeHash {
		hashValues[i] = cadence.UInt8(b)
	}

	event := cadence.NewEvent([]cadence.Value{
		cadence.NewAddress(address),
		cadence.NewArray(hashValues).WithType(cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)),
		cadence.String(contractName),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType("flow.AccountContractAdded"),
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

// makeAccountContractUpdatedEvent builds a CCF-encoded flow.AccountContractUpdated event.
func makeAccountContractUpdatedEvent(
	t *testing.T,
	address flow.Address,
	contractName string,
	codeHash []byte,
	txIndex uint32,
	eventIndex uint32,
) flow.Event {
	t.Helper()

	eventType := cadence.NewEventType(
		nil,
		"flow.AccountContractUpdated",
		[]cadence.Field{
			{Identifier: "address", Type: cadence.AddressType},
			{Identifier: "codeHash", Type: cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)},
			{Identifier: "contract", Type: cadence.StringType},
		},
		nil,
	)

	hashValues := make([]cadence.Value, 32)
	for i, b := range codeHash {
		hashValues[i] = cadence.UInt8(b)
	}

	event := cadence.NewEvent([]cadence.Value{
		cadence.NewAddress(address),
		cadence.NewArray(hashValues).WithType(cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)),
		cadence.String(contractName),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType("flow.AccountContractUpdated"),
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

// makeAccountContractRemovedEvent builds a CCF-encoded flow.AccountContractRemoved event.
func makeAccountContractRemovedEvent(
	t *testing.T,
	address flow.Address,
	contractName string,
	codeHash []byte,
	txIndex uint32,
	eventIndex uint32,
) flow.Event {
	t.Helper()

	eventType := cadence.NewEventType(
		nil,
		"flow.AccountContractRemoved",
		[]cadence.Field{
			{Identifier: "address", Type: cadence.AddressType},
			{Identifier: "codeHash", Type: cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)},
			{Identifier: "contract", Type: cadence.StringType},
		},
		nil,
	)

	hashValues := make([]cadence.Value, 32)
	for i, b := range codeHash {
		hashValues[i] = cadence.UInt8(b)
	}

	event := cadence.NewEvent([]cadence.Value{
		cadence.NewAddress(address),
		cadence.NewArray(hashValues).WithType(cadence.NewConstantSizedArrayType(32, cadence.UInt8Type)),
		cadence.String(contractName),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType("flow.AccountContractRemoved"),
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

// cadenceCodeHash computes the SHA3-256 hash of contract code using the same algorithm
// that cadence uses internally (stdlib.CodeToHashValue).
func cadenceCodeHash(code []byte) []byte {
	h := sha3.Sum256(code)
	return h[:]
}
