package extended_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/onflow/cadence"
	cadenceccf "github.com/onflow/cadence/encoding/ccf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	fvmsnapshot "github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	executionmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	iterutil "github.com/onflow/flow-go/storage/indexes/iterator"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"

	. "github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
)

// contractsBootstrapHeight is the height of the first indexed block.
// The index bootstraps at this height (loadDeployedContracts is called here).
const contractsBootstrapHeight = uint64(100)

// contractsEventHeight is the height of the block that contains contract events.
// Events at this height are processed AFTER the index is bootstrapped.
const contractsEventHeight = uint64(101)

// collectContractPage is a test helper that creates an iterator from the store and collects
// results via CollectResults, returning the deployments slice. No filter or cursor is applied.
func collectContractPage(t *testing.T, iter storage.ContractDeploymentIterator, limit uint32) []access.ContractDeployment {
	t.Helper()
	collected, _, err := iterutil.CollectResults(iter, limit, nil)
	require.NoError(t, err)
	return collected
}

// ===== Happy-path tests =====

// TestContractsIndexer_NoEvents verifies that indexing a block with no contract events
// stores no deployments and advances the latest indexed height.
func TestContractsIndexer_NoEvents(t *testing.T) {
	t.Parallel()

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	// loadDeployedContracts is called; empty snapshot → no pre-existing contracts.
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()

	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{},
	})

	latest, err := store.LatestIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, contractsBootstrapHeight, latest)

	iter, err := store.All(nil)
	require.NoError(t, err)
	deployments := collectContractPage(t, iter, 1)
	assert.Empty(t, deployments)
}

// TestContractsIndexer_NextHeight_NotBootstrapped verifies that NextHeight returns the
// configured first height before any blocks have been indexed.
func TestContractsIndexer_NextHeight_NotBootstrapped(t *testing.T) {
	t.Parallel()

	// No script executor needed — NextHeight() only touches the store, not the executor.
	indexer, _, _, _ := newContractsIndexerNilExecutor(t, flow.Testnet, contractsBootstrapHeight)

	height, err := indexer.NextHeight()
	require.NoError(t, err)
	assert.Equal(t, contractsBootstrapHeight, height)
}

// TestContractsIndexer_ContractAdded verifies that an AccountContractAdded event creates
// a deployment entry with all fields correctly stored.
func TestContractsIndexer_ContractAdded(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "MyContract"
	code := []byte("access(all) contract MyContract {}")
	codeHash := access.CadenceCodeHash(code)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	// Step 1: bootstrap block — no events, empty snapshot.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	// Step 2: event block — contract deployed.
	deployingTxID := unittest.IdentifierFixture()
	eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
	snap := makeContractSnapshot(t, address, map[string][]byte{contractName: code})
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).
		Return(snap, nil).Once()

	event := makeAccountContractAddedEvent(t, address, contractName, codeHash, 0, 0)
	event.TransactionID = deployingTxID
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: eventHeader, Events: []flow.Event{event}})

	contractID := fmt.Sprintf("A.%s.%s", address.Hex(), contractName)
	deployment, err := store.ByContractID(contractID)
	require.NoError(t, err)

	assert.Equal(t, contractID, deployment.ContractID)
	assert.Equal(t, address, deployment.Address)
	assert.Equal(t, contractsEventHeight, deployment.BlockHeight)
	assert.Equal(t, deployingTxID, deployment.TransactionID)
	assert.Equal(t, uint32(0), deployment.TransactionIndex)
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
	codeHashV1 := access.CadenceCodeHash(codeV1)
	codeHashV2 := access.CadenceCodeHash(codeV2)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	// Bootstrap block.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	// Height 101: AccountContractAdded
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
	snap1 := makeContractSnapshot(t, address, map[string][]byte{contractName: codeV1})
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).
		Return(snap1, nil).Once()
	addEvent := makeAccountContractAddedEvent(t, address, contractName, codeHashV1, 0, 0)
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header1, Events: []flow.Event{addEvent}})

	// Height 102: AccountContractUpdated
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight+1))
	snap2 := makeContractSnapshot(t, address, map[string][]byte{contractName: codeV2})
	updateTxID := unittest.IdentifierFixture()
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight+1).
		Return(snap2, nil).Once()
	updateEvent := makeAccountContractUpdatedEvent(t, address, contractName, codeHashV2, 1, 0)
	updateEvent.TransactionID = updateTxID
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header2, Events: []flow.Event{updateEvent}})

	contractID := fmt.Sprintf("A.%s.%s", address.Hex(), contractName)

	// ByContractID returns the most recent deployment (height 102).
	latest, err := store.ByContractID(contractID)
	require.NoError(t, err)
	assert.Equal(t, contractsEventHeight+1, latest.BlockHeight)
	assert.Equal(t, codeV2, latest.Code)
	assert.Equal(t, updateTxID, latest.TransactionID)

	// DeploymentsByContractID returns both deployments, most recent first.
	depIter, err := store.DeploymentsByContractID(contractID, nil)
	require.NoError(t, err)
	deployments := collectContractPage(t, depIter, 10)
	require.Len(t, deployments, 2)
	assert.Equal(t, contractsEventHeight+1, deployments[0].BlockHeight)
	assert.Equal(t, contractsEventHeight, deployments[1].BlockHeight)
}

// TestContractsIndexer_MultipleContractsInOneBlock verifies that multiple AccountContractAdded
// events in a single block each create a separate deployment entry.
func TestContractsIndexer_MultipleContractsInOneBlock(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()
	code1 := []byte("access(all) contract Alpha {}")
	code2 := []byte("access(all) contract Beta {}")
	hash1 := access.CadenceCodeHash(code1)
	hash2 := access.CadenceCodeHash(code2)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	// Bootstrap block.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	// Event block: two contracts deployed in the same block.
	// contractRetriever caches the snapshot per-block, so GetStorageSnapshot is called once.
	// The snapshot must contain both accounts.
	snap := makeMultiAccountSnapshot(t,
		addr1, map[string][]byte{"Alpha": code1},
		addr2, map[string][]byte{"Beta": code2},
	)
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).
		Return(snap, nil).Once()

	eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
	event1 := makeAccountContractAddedEvent(t, addr1, "Alpha", hash1, 0, 0)
	event2 := makeAccountContractAddedEvent(t, addr2, "Beta", hash2, 1, 0)
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: eventHeader,
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
	allIter, err := store.All(nil)
	require.NoError(t, err)
	assert.Len(t, collectContractPage(t, allIter, 10), 2)
}

// TestContractsIndexer_SameAccountCached verifies that when one block deploys two contracts
// to the same account, GetStorageSnapshot is called only once (result is cached).
func TestContractsIndexer_SameAccountCached(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	codeA := []byte("access(all) contract Alpha {}")
	codeB := []byte("access(all) contract Beta {}")
	hashA := access.CadenceCodeHash(codeA)
	hashB := access.CadenceCodeHash(codeB)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	// Bootstrap block.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	// Event block: GetStorageSnapshot must be called exactly once (caching).
	eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
	snap := makeContractSnapshot(t, address, map[string][]byte{"Alpha": codeA, "Beta": codeB})
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).
		Return(snap, nil).Once()

	eventA := makeAccountContractAddedEvent(t, address, "Alpha", hashA, 0, 0)
	eventB := makeAccountContractAddedEvent(t, address, "Beta", hashB, 0, 1)
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: eventHeader,
		Events: []flow.Event{eventA, eventB},
	})

	byAddrIter, err := store.ByAddress(address, nil)
	require.NoError(t, err)
	assert.Len(t, collectContractPage(t, byAddrIter, 10), 2)
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
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	// Bootstrap block.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	// Block 101: addr1 deploys Foo.
	snap1 := makeContractSnapshot(t, addr1, map[string][]byte{"Foo": code1})
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).
		Return(snap1, nil).Once()
	header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header1,
		Events: []flow.Event{makeAccountContractAddedEvent(t, addr1, "Foo", access.CadenceCodeHash(code1), 0, 0)},
	})

	// Block 102: addr2 deploys Bar.
	snap2 := makeContractSnapshot(t, addr2, map[string][]byte{"Bar": code2})
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight+1).
		Return(snap2, nil).Once()
	header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight+1))
	indexContractsBlock(t, indexer, lm, db, BlockData{
		Header: header2,
		Events: []flow.Event{makeAccountContractAddedEvent(t, addr2, "Bar", access.CadenceCodeHash(code2), 0, 0)},
	})

	// ByAddress(addr1) returns only Foo.
	iter1, err := store.ByAddress(addr1, nil)
	require.NoError(t, err)
	deps1 := collectContractPage(t, iter1, 10)
	require.Len(t, deps1, 1)
	assert.Equal(t, fmt.Sprintf("A.%s.Foo", addr1.Hex()), deps1[0].ContractID)

	// ByAddress(addr2) returns only Bar.
	iter2, err := store.ByAddress(addr2, nil)
	require.NoError(t, err)
	deps2 := collectContractPage(t, iter2, 10)
	require.Len(t, deps2, 1)
	assert.Equal(t, fmt.Sprintf("A.%s.Bar", addr2.Hex()), deps2[0].ContractID)
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
	hashV1 := access.CadenceCodeHash(codeV1)
	hashV2 := access.CadenceCodeHash(codeV2)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	firstHeight := contractsBootstrapHeight
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, firstHeight, scriptExecutor)

	// Height 100 (firstHeight): bootstrap with no events. loadDeployedContracts is called.
	header100 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight))
	scriptExecutor.On("GetStorageSnapshot", firstHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header100, Events: []flow.Event{}})

	first, err := store.FirstIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, firstHeight, first)

	// Height 101: contract added — this represents a backfilled block.
	header101 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight+1))
	snap1 := makeContractSnapshot(t, address, map[string][]byte{contractName: codeV1})
	scriptExecutor.On("GetStorageSnapshot", firstHeight+1).
		Return(snap1, nil).Once()

	addTxID := unittest.IdentifierFixture()
	addEvent := makeAccountContractAddedEvent(t, address, contractName, hashV1, 2, 0)
	addEvent.TransactionID = addTxID
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header101, Events: []flow.Event{addEvent}})

	// Height 102: contract updated.
	header102 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight+2))
	snap2 := makeContractSnapshot(t, address, map[string][]byte{contractName: codeV2})
	scriptExecutor.On("GetStorageSnapshot", firstHeight+2).
		Return(snap2, nil).Once()

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
	depIter, err := store.DeploymentsByContractID(contractID, nil)
	require.NoError(t, err)
	deps := collectContractPage(t, depIter, 10)
	require.Len(t, deps, 2)
	assert.Equal(t, firstHeight+2, deps[0].BlockHeight)
	assert.Equal(t, firstHeight+1, deps[1].BlockHeight)
	assert.Equal(t, addTxID, deps[1].TransactionID)
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
		hashes[i] = access.CadenceCodeHash(codes[i])
	}

	scriptExecutor := executionmock.NewScriptExecutor(t)
	firstHeight := contractsBootstrapHeight
	indexer, store, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, firstHeight, scriptExecutor)

	// Root block: no events, but loadDeployedContracts is called.
	scriptExecutor.On("GetStorageSnapshot", firstHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	root := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight))
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: root, Events: []flow.Event{}})

	// Index three blocks, one contract added per block.
	for i := range 3 {
		h := firstHeight + uint64(i+1)
		contractName := fmt.Sprintf("C%d", i)
		snap := makeContractSnapshot(t, addrs[i], map[string][]byte{contractName: codes[i]})
		scriptExecutor.On("GetStorageSnapshot", h).Return(snap, nil).Once()

		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(h))
		event := makeAccountContractAddedEvent(t, addrs[i], contractName, hashes[i], 0, 0)
		indexContractsBlock(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{event}})
	}

	latest, err := store.LatestIndexedHeight()
	require.NoError(t, err)
	assert.Equal(t, firstHeight+3, latest)

	// All() returns all three contracts.
	allIter, err := store.All(nil)
	require.NoError(t, err)
	assert.Len(t, collectContractPage(t, allIter, 10), 3)
}

// ===== Error and edge-case tests =====

// TestContractsIndexer_AlreadyIndexed verifies that attempting to index the same height
// twice returns ErrAlreadyIndexed.
func TestContractsIndexer_AlreadyIndexed(t *testing.T) {
	t.Parallel()

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, _, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))

	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{}})

	err := indexContractsBlockExpectError(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{}})
	require.ErrorIs(t, err, ErrAlreadyIndexed)
}

// TestContractsIndexer_ContractRemoved verifies that an AccountContractRemoved event
// returns an error (contract removal is not supported). collectDeployments fails before
// loadDeployedContracts, so no script executor is needed.
func TestContractsIndexer_ContractRemoved(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	// collectDeployments errors for ContractRemoved before loadDeployedContracts runs,
	// so no GetStorageSnapshot call is made — use nil executor.
	indexer, _, lm, db := newContractsIndexerNilExecutor(t, flow.Testnet, contractsBootstrapHeight)
	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))

	removedEvent := makeAccountContractRemovedEvent(t, address, "SomeContract", make([]byte, 32), 0, 0)

	err := indexContractsBlockExpectError(t, indexer, lm, db, BlockData{
		Header: header,
		Events: []flow.Event{removedEvent},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// TestContractsIndexer_CodeHashMismatch verifies that when the code fetched from the snapshot
// does not match the hash in the event, IndexBlockData returns an error.
func TestContractsIndexer_CodeHashMismatch(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "Mismatch"
	eventCode := []byte("access(all) contract Mismatch {}")
	differentCode := []byte("access(all) contract Different {}")
	// Event carries hash of eventCode, but the snapshot holds differentCode.
	eventHash := access.CadenceCodeHash(eventCode)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, _, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	// Bootstrap block.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	// Event block: snapshot has differentCode, but event hash is for eventCode.
	eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
	snap := makeContractSnapshot(t, address, map[string][]byte{contractName: differentCode})
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(snap, nil).Once()

	event := makeAccountContractAddedEvent(t, address, contractName, eventHash, 0, 0)
	err := indexContractsBlockExpectError(t, indexer, lm, db, BlockData{
		Header: eventHeader,
		Events: []flow.Event{event},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code hash mismatch")
}

// TestContractsIndexer_ScriptExecutorError verifies that an error from GetStorageSnapshot
// is propagated and causes IndexBlockData to fail.
func TestContractsIndexer_ScriptExecutorError(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	scriptErr := fmt.Errorf("storage unavailable")

	scriptExecutor := executionmock.NewScriptExecutor(t)
	indexer, _, lm, db := newContractsIndexerWithScriptExecutor(t, flow.Testnet, contractsBootstrapHeight, scriptExecutor)

	// Bootstrap block.
	bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).
		Return(fvmsnapshot.MapStorageSnapshot{}, nil).Once()
	indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})

	// Event block: GetStorageSnapshot fails.
	eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
	scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(nil, scriptErr).Once()

	codeHash := access.CadenceCodeHash([]byte("access(all) contract MyContract {}"))
	event := makeAccountContractAddedEvent(t, address, "MyContract", codeHash, 0, 0)

	err := indexContractsBlockExpectError(t, indexer, lm, db, BlockData{
		Header: eventHeader,
		Events: []flow.Event{event},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, scriptErr)
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
		storeErr := fmt.Errorf("unexpected storage error")
		// Store is already initialized so loadDeployedContracts is skipped.
		mockStore.On("UninitializedFirstHeight").Return(testHeight, true)
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

// newContractsIndexerNilExecutor creates a Contracts indexer with a nil script executor.
// Use this for tests where GetStorageSnapshot will never be called:
//   - tests that don't call IndexBlockData, OR
//   - tests where collectDeployments errors before loadDeployedContracts runs.
func newContractsIndexerNilExecutor(
	t *testing.T,
	chainID flow.ChainID,
	firstHeight uint64,
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
	indexer := NewContracts(unittest.Logger(), chainID.Chain(), store, nil, &metrics.NoopCollector{})
	return indexer, store, lm, db
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

// makeContractSnapshot builds a MapStorageSnapshot containing a single account with the
// given contracts. The snapshot can be used to satisfy GetStorageSnapshot for a given height.
func makeContractSnapshot(t *testing.T, address flow.Address, contracts map[string][]byte) fvmsnapshot.MapStorageSnapshot {
	t.Helper()
	snap := make(fvmsnapshot.MapStorageSnapshot)

	// Mark account as existing via a valid account status register.
	status := environment.NewAccountStatus()
	snap[flow.AccountStatusRegisterID(address)] = status.ToBytes()

	// Encode and store contract names.
	names := make([]string, 0, len(contracts))
	for name := range contracts {
		names = append(names, name)
	}
	var nameBuf bytes.Buffer
	require.NoError(t, cbor.NewEncoder(&nameBuf).Encode(names))
	snap[flow.ContractNamesRegisterID(address)] = nameBuf.Bytes()

	// Store each contract's code.
	for name, code := range contracts {
		snap[flow.ContractRegisterID(address, name)] = flow.RegisterValue(code)
	}
	return snap
}

// makeMultiAccountSnapshot builds a MapStorageSnapshot containing two accounts with contracts.
func makeMultiAccountSnapshot(
	t *testing.T,
	addr1 flow.Address, contracts1 map[string][]byte,
	addr2 flow.Address, contracts2 map[string][]byte,
) fvmsnapshot.MapStorageSnapshot {
	t.Helper()
	snap := make(fvmsnapshot.MapStorageSnapshot)
	for k, v := range makeContractSnapshot(t, addr1, contracts1) {
		snap[k] = v
	}
	for k, v := range makeContractSnapshot(t, addr2, contracts2) {
		snap[k] = v
	}
	return snap
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
	return makeContractEvent(t, "flow.AccountContractAdded", address, contractName, codeHash, txIndex, eventIndex)
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
	return makeContractEvent(t, "flow.AccountContractUpdated", address, contractName, codeHash, txIndex, eventIndex)
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
	return makeContractEvent(t, "flow.AccountContractRemoved", address, contractName, codeHash, txIndex, eventIndex)
}

func makeContractEvent(
	t *testing.T,
	eventTypeName string,
	address flow.Address,
	contractName string,
	codeHash []byte,
	txIndex uint32,
	eventIndex uint32,
) flow.Event {
	t.Helper()

	eventType := cadence.NewEventType(
		nil,
		eventTypeName,
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

	payload, err := cadenceccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType(eventTypeName),
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}
