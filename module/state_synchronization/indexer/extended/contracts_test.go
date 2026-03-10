package extended_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/fxamacker/cbor/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
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

const (
	// contractsBootstrapHeight is the height of the first indexed block.
	// The index bootstraps at this height (loadDeployedContracts is called here).
	contractsBootstrapHeight = uint64(100)

	// contractsEventHeight is the height of the block that contains contract events.
	// Events at this height are processed AFTER the index is bootstrapped.
	contractsEventHeight = uint64(101)

	// Contract event type names used to build test events.
	eventContractAdded   = "flow.AccountContractAdded"
	eventContractUpdated = "flow.AccountContractUpdated"
	eventContractRemoved = "flow.AccountContractRemoved"
)

// collectContractPage is a test helper that creates an iterator from the store and collects
// results via CollectResults, returning the deployments slice. No filter or cursor is applied.
func collectContractPage(t *testing.T, iter storage.ContractDeploymentIterator, limit uint32) []access.ContractDeployment {
	t.Helper()
	collected, _, err := iterutil.CollectResults(iter, limit, nil)
	require.NoError(t, err)
	return collected
}

// ===== Happy-path tests =====

// TestContractsIndexer_NextHeight_NotBootstrapped verifies that NextHeight returns the
// configured first height before any blocks have been indexed.
func TestContractsIndexer_NextHeight_NotBootstrapped(t *testing.T) {
	t.Parallel()

	runWithContractsIndexer(t, contractsBootstrapHeight, nil, nil, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, _ storage.LockManager, _ storage.DB) {
		height, err := indexer.NextHeight()
		require.NoError(t, err)
		assert.Equal(t, contractsBootstrapHeight, height)
	})
}

// TestContractsIndexer_ContractAdded verifies that an AccountContractAdded event creates
// a deployment entry with all fields correctly stored.
func TestContractsIndexer_ContractAdded(t *testing.T) {
	t.Parallel()

	deployingTxID := unittest.IdentifierFixture()
	address := unittest.RandomAddressFixture()
	contractName := "MyContract"
	code := []byte("access(all) contract MyContract {}")
	codeHash := access.CadenceCodeHash(code)

	expected := access.ContractDeployment{
		Address:          address,
		ContractName:     contractName,
		BlockHeight:      contractsEventHeight,
		TransactionID:    deployingTxID,
		TransactionIndex: 0,
		EventIndex:       0,
		Code:             code,
		CodeHash:         codeHash,
	}

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, store storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Step 1: bootstrap block — no events, empty snapshot.
		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})
		require.NoError(t, err)

		// Step 2: event block — contract deployed.
		eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		snap := makeContractSnapshot(t, address, map[string][]byte{contractName: code})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).
			Return(snap, nil).Once()

		event := makeContractEvent(t, eventContractAdded, address, contractName, codeHash, 0, 0)
		event.TransactionID = deployingTxID
		err = indexContractsBlock(t, indexer, lm, db, BlockData{Header: eventHeader, Events: []flow.Event{event}})
		require.NoError(t, err)

		deployment, err := store.ByContract(address, contractName)
		require.NoError(t, err)

		assertContractDeployment(t, expected, deployment)
	})
}

// TestContractsIndexer_ContractUpdated verifies that an AccountContractUpdated event at a
// subsequent block creates a new deployment entry for the same contract.
func TestContractsIndexer_ContractAddedThenUpdated(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "MyContract"
	codeV1 := []byte("access(all) contract MyContract { let x: Int; init() { self.x = 1 } }")
	codeV2 := []byte("access(all) contract MyContract { let x: Int; init() { self.x = 2 } }")
	codeHashV1 := access.CadenceCodeHash(codeV1)
	codeHashV2 := access.CadenceCodeHash(codeV2)

	updateTxID := unittest.IdentifierFixture()
	addTxID := unittest.IdentifierFixture()

	expectedAdd := access.ContractDeployment{
		Address:       address,
		ContractName:  contractName,
		BlockHeight:   contractsEventHeight,
		TransactionID: addTxID,
		Code:          codeV1,
		CodeHash:      codeHashV1,
	}
	expectedUpdate := access.ContractDeployment{
		Address:          address,
		ContractName:     contractName,
		BlockHeight:      contractsEventHeight + 1,
		TransactionID:    updateTxID,
		TransactionIndex: 1,
		Code:             codeV2,
		CodeHash:         codeHashV2,
	}

	addEvent := makeContractEvent(t, eventContractAdded, address, contractName, codeHashV1, 0, 0)
	addEvent.TransactionID = addTxID

	updateEvent := makeContractEvent(t, eventContractUpdated, address, contractName, codeHashV2, 1, 0)
	updateEvent.TransactionID = updateTxID

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, store storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Bootstrap block.
		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})
		require.NoError(t, err)

		firstHeight, err := store.FirstIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, contractsBootstrapHeight, firstHeight)

		allIter, err := store.All(nil)
		require.NoError(t, err)
		assert.Empty(t, collectContractPage(t, allIter, 1))

		// Height 101: AccountContractAdded
		header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		snap1 := makeContractSnapshot(t, address, map[string][]byte{contractName: codeV1})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(snap1, nil).Once()

		err = indexContractsBlock(t, indexer, lm, db, BlockData{Header: header1, Events: []flow.Event{addEvent}})
		require.NoError(t, err)

		// Height 102: AccountContractUpdated
		header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight+1))
		snap2 := makeContractSnapshot(t, address, map[string][]byte{contractName: codeV2})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight+1).Return(snap2, nil).Once()

		err = indexContractsBlock(t, indexer, lm, db, BlockData{Header: header2, Events: []flow.Event{updateEvent}})
		require.NoError(t, err)

		latestHeight, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, contractsEventHeight+1, latestHeight)

		// ByContract returns the most recent deployment (height 102).
		latest, err := store.ByContract(address, contractName)
		require.NoError(t, err)
		assertContractDeployment(t, expectedUpdate, latest)

		// DeploymentsByContract returns both deployments, most recent first.
		depIter, err := store.DeploymentsByContract(address, contractName, nil)
		require.NoError(t, err)
		deployments := collectContractPage(t, depIter, 10)
		require.Len(t, deployments, 2)
		assertContractDeployment(t, expectedUpdate, deployments[0])
		assertContractDeployment(t, expectedAdd, deployments[1])
	})
}

// TestContractsIndexer_MultipleContractsInOneBlock verifies that multiple AccountContractAdded
// events in a single block each produce a separate deployment entry. It covers two sub-cases:
// two contracts on the same address (exercising per-block snapshot caching — GetStorageSnapshot
// must be called only once for that address) and one contract on a different address.
func TestContractsIndexer_MultipleContractsInOneBlock(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()
	codeA := []byte("access(all) contract Alpha {}")
	codeB := []byte("access(all) contract Beta {}")
	codeG := []byte("access(all) contract Gamma {}")
	hashA := access.CadenceCodeHash(codeA)
	hashB := access.CadenceCodeHash(codeB)
	hashG := access.CadenceCodeHash(codeG)

	expectedAlpha := access.ContractDeployment{
		Address:      addr1,
		ContractName: "Alpha",
		BlockHeight:  contractsEventHeight,
		Code:         codeA,
		CodeHash:     hashA,
	}
	expectedBeta := access.ContractDeployment{
		Address:      addr1,
		ContractName: "Beta",
		BlockHeight:  contractsEventHeight,
		EventIndex:   1,
		Code:         codeB,
		CodeHash:     hashB,
	}
	expectedGamma := access.ContractDeployment{
		Address:          addr2,
		ContractName:     "Gamma",
		BlockHeight:      contractsEventHeight,
		TransactionIndex: 1,
		Code:             codeG,
		CodeHash:         hashG,
	}

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, store storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Bootstrap block.
		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})
		require.NoError(t, err)

		// Event block: addr1 deploys Alpha and Beta (same account, caching exercised),
		// addr2 deploys Gamma. GetStorageSnapshot is called exactly once per the .Once() mock.
		snap := makeMultiAccountSnapshot(t,
			addr1, map[string][]byte{"Alpha": codeA, "Beta": codeB},
			addr2, map[string][]byte{"Gamma": codeG},
		)
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(snap, nil).Once()

		eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		eventA := makeContractEvent(t, eventContractAdded, addr1, "Alpha", hashA, 0, 0)
		eventB := makeContractEvent(t, eventContractAdded, addr1, "Beta", hashB, 0, 1)
		eventG := makeContractEvent(t, eventContractAdded, addr2, "Gamma", hashG, 1, 0)
		err = indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: eventHeader,
			Events: []flow.Event{eventA, eventB, eventG},
		})
		require.NoError(t, err)

		alpha, err := store.ByContract(addr1, "Alpha")
		require.NoError(t, err)
		assertContractDeployment(t, expectedAlpha, alpha)

		beta, err := store.ByContract(addr1, "Beta")
		require.NoError(t, err)
		assertContractDeployment(t, expectedBeta, beta)

		gamma, err := store.ByContract(addr2, "Gamma")
		require.NoError(t, err)
		assertContractDeployment(t, expectedGamma, gamma)

		// ByAddress returns only contracts for the requested account.
		addr1Iter, err := store.ByAddress(addr1, nil)
		require.NoError(t, err)
		assert.Len(t, collectContractPage(t, addr1Iter, 10), 2)

		addr2Iter, err := store.ByAddress(addr2, nil)
		require.NoError(t, err)
		assert.Len(t, collectContractPage(t, addr2Iter, 10), 1)

		// All() returns all three contracts.
		allIter, err := store.All(nil)
		require.NoError(t, err)
		assert.Len(t, collectContractPage(t, allIter, 10), 3)
	})
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
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, store storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Bootstrap block.
		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})
		require.NoError(t, err)

		// Block 101: addr1 deploys Foo.
		snap1 := makeContractSnapshot(t, addr1, map[string][]byte{"Foo": code1})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).
			Return(snap1, nil).Once()
		header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		err = indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: header1,
			Events: []flow.Event{makeContractEvent(t, eventContractAdded, addr1, "Foo", access.CadenceCodeHash(code1), 0, 0)},
		})
		require.NoError(t, err)

		// Block 102: addr2 deploys Bar.
		snap2 := makeContractSnapshot(t, addr2, map[string][]byte{"Bar": code2})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight+1).
			Return(snap2, nil).Once()
		header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight+1))
		err = indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: header2,
			Events: []flow.Event{makeContractEvent(t, eventContractAdded, addr2, "Bar", access.CadenceCodeHash(code2), 0, 0)},
		})
		require.NoError(t, err)

		// ByAddress(addr1) returns only Foo.
		iter1, err := store.ByAddress(addr1, nil)
		require.NoError(t, err)
		deps1 := collectContractPage(t, iter1, 10)
		require.Len(t, deps1, 1)
		assert.Equal(t, addr1, deps1[0].Address)
		assert.Equal(t, "Foo", deps1[0].ContractName)

		// ByAddress(addr2) returns only Bar.
		iter2, err := store.ByAddress(addr2, nil)
		require.NoError(t, err)
		deps2 := collectContractPage(t, iter2, 10)
		require.Len(t, deps2, 1)
		assert.Equal(t, addr2, deps2[0].Address)
		assert.Equal(t, "Bar", deps2[0].ContractName)
	})
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
	runWithContractsIndexer(t, firstHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, store storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Root block: no events, but loadDeployedContracts is called.
		root := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(firstHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: root, Events: []flow.Event{}})
		require.NoError(t, err)

		// Index three blocks, one contract added per block.
		for i := range 3 {
			h := firstHeight + uint64(i+1)
			contractName := fmt.Sprintf("C%d", i)
			snap := makeContractSnapshot(t, addrs[i], map[string][]byte{contractName: codes[i]})
			scriptExecutor.On("GetStorageSnapshot", h).Return(snap, nil).Once()

			header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(h))
			event := makeContractEvent(t, eventContractAdded, addrs[i], contractName, hashes[i], 0, 0)
			err = indexContractsBlock(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{event}})
			require.NoError(t, err)
		}

		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, firstHeight+3, latest)

		// All() returns all three contracts.
		allIter, err := store.All(nil)
		require.NoError(t, err)
		assert.Len(t, collectContractPage(t, allIter, 10), 3)
	})
}

// TestContractsIndexer_Bootstrap_PreExistingContracts verifies that loadDeployedContracts
// finds contracts pre-loaded into the fakeRegisters and stores placeholder deployments for
// them, while skipping any contract already covered by the bootstrap block's own events.
func TestContractsIndexer_Bootstrap_PreExistingContracts(t *testing.T) {
	t.Parallel()

	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()
	codeAlpha := []byte("access(all) contract Alpha {}")
	codeBeta := []byte("access(all) contract Beta {}")
	codeGamma := []byte("access(all) contract Gamma {}")

	// addr1 has Alpha and Beta pre-deployed; addr2 has Gamma. Alpha is also in the
	// bootstrap block's events, so it should NOT get a placeholder.
	registers := &fakeRegisters{
		Entries: []flow.RegisterEntry{
			{Key: flow.ContractRegisterID(addr1, "Alpha"), Value: codeAlpha},
			{Key: flow.ContractRegisterID(addr1, "Beta"), Value: codeBeta},
			{Key: flow.ContractRegisterID(addr2, "Gamma"), Value: codeGamma},
		},
	}

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, registers, scriptExecutor, func(indexer *Contracts, store storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Bootstrap block also deploys Alpha — that deployment gets a real record, not a placeholder.
		alphaHash := access.CadenceCodeHash(codeAlpha)
		snap := makeContractSnapshot(t, addr1, map[string][]byte{"Alpha": codeAlpha})
		scriptExecutor.On("GetStorageSnapshot", contractsBootstrapHeight).Return(snap, nil).Once()

		// loadDeployedContracts verifies the code-register scan against the contract names register.
		scriptExecutor.On("RegisterValue", flow.ContractNamesRegisterID(addr1), contractsBootstrapHeight).
			Return(encodeContractNames(t, "Alpha", "Beta"), nil)
		scriptExecutor.On("RegisterValue", flow.ContractNamesRegisterID(addr2), contractsBootstrapHeight).
			Return(encodeContractNames(t, "Gamma"), nil)

		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		alphaEvent := makeContractEvent(t, eventContractAdded, addr1, "Alpha", alphaHash, 0, 0)
		err := indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: bootstrapHeader,
			Events: []flow.Event{alphaEvent},
		})
		require.NoError(t, err)

		// Alpha: real deployment from the event — not a placeholder.
		alpha, err := store.ByContract(addr1, "Alpha")
		require.NoError(t, err)
		assertContractDeployment(t, access.ContractDeployment{
			Address:      addr1,
			ContractName: "Alpha",
			BlockHeight:  contractsBootstrapHeight,
			Code:         codeAlpha,
			CodeHash:     alphaHash,
		}, alpha)
		assert.False(t, alpha.IsPlaceholder)

		// Beta: placeholder from loadDeployedContracts.
		beta, err := store.ByContract(addr1, "Beta")
		require.NoError(t, err)
		assertContractDeployment(t, access.ContractDeployment{
			Address:       addr1,
			ContractName:  "Beta",
			BlockHeight:   contractsBootstrapHeight,
			Code:          codeBeta,
			CodeHash:      access.CadenceCodeHash(codeBeta),
			IsPlaceholder: true,
		}, beta)

		// Gamma: placeholder from loadDeployedContracts.
		gamma, err := store.ByContract(addr2, "Gamma")
		require.NoError(t, err)
		assertContractDeployment(t, access.ContractDeployment{
			Address:       addr2,
			ContractName:  "Gamma",
			BlockHeight:   contractsBootstrapHeight,
			Code:          codeGamma,
			CodeHash:      access.CadenceCodeHash(codeGamma),
			IsPlaceholder: true,
		}, gamma)

		// All() returns all three contracts.
		allIter, err := store.All(nil)
		require.NoError(t, err)
		assert.Len(t, collectContractPage(t, allIter, 10), 3)
	})
}

// TestContractsIndexer_Bootstrap_MarksDeletedContracts verifies that loadDeployedContracts loads
// all code registers (including those with empty values representing deleted contracts) and marks
// them as deleted if their name is absent from the contract names register. The contract names
// register is the source of truth: contracts present in code registers but absent from the names
// register are indexed with IsDeleted=true.
func TestContractsIndexer_Bootstrap_MarksDeletedContracts(t *testing.T) {
	t.Parallel()

	addr := unittest.RandomAddressFixture()
	codeAlpha := []byte("access(all) contract Alpha {}")
	codeBeta := []byte("access(all) contract Beta {}")
	codeDeleted := []byte{}

	// Alpha and Beta are live; Deleted was removed (empty value in pebble).
	registers := &fakeRegisters{
		Entries: []flow.RegisterEntry{
			{Key: flow.ContractRegisterID(addr, "Alpha"), Value: codeAlpha},
			{Key: flow.ContractRegisterID(addr, "Beta"), Value: codeBeta},
			{Key: flow.ContractRegisterID(addr, "Deleted"), Value: codeDeleted},
		},
	}

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, registers, scriptExecutor, func(indexer *Contracts, store storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// The contract names register only contains the two live contracts; the deleted one
		// was already removed from it when the contract was removed.
		scriptExecutor.On("RegisterValue", flow.ContractNamesRegisterID(addr), contractsBootstrapHeight).
			Return(encodeContractNames(t, "Alpha", "Beta"), nil)

		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: bootstrapHeader,
			Events: []flow.Event{},
		})
		require.NoError(t, err)

		// Alpha and Beta get placeholder deployments.
		alpha, err := store.ByContract(addr, "Alpha")
		require.NoError(t, err)
		assertContractDeployment(t, access.ContractDeployment{
			Address:       addr,
			ContractName:  "Alpha",
			BlockHeight:   contractsBootstrapHeight,
			Code:          codeAlpha,
			CodeHash:      access.CadenceCodeHash(codeAlpha),
			IsPlaceholder: true,
		}, alpha)

		beta, err := store.ByContract(addr, "Beta")
		require.NoError(t, err)
		assertContractDeployment(t, access.ContractDeployment{
			Address:       addr,
			ContractName:  "Beta",
			BlockHeight:   contractsBootstrapHeight,
			Code:          codeBeta,
			CodeHash:      access.CadenceCodeHash(codeBeta),
			IsPlaceholder: true,
		}, beta)

		// Deleted contract is indexed but marked as deleted.
		deleted, err := store.ByContract(addr, "Deleted")
		require.NoError(t, err)
		assertContractDeployment(t, access.ContractDeployment{
			Address:       addr,
			ContractName:  "Deleted",
			BlockHeight:   contractsBootstrapHeight,
			Code:          codeDeleted,
			CodeHash:      access.CadenceCodeHash(codeDeleted),
			IsPlaceholder: true,
			IsDeleted:     true,
		}, deleted)

		// All() returns all three contracts (including deleted).
		allIter, err := store.All(nil)
		require.NoError(t, err)
		assert.Len(t, collectContractPage(t, allIter, 10), 3)
	})
}

// ===== ProcessBlockData tests =====

// TestContractsIndexer_ProcessBlockData_NoEvents verifies that ProcessBlockData returns
// empty entries and zero counts for a block with no contract events.
func TestContractsIndexer_ProcessBlockData_NoEvents(t *testing.T) {
	t.Parallel()

	runWithContractsIndexer(t, contractsBootstrapHeight, nil, nil, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, _ storage.LockManager, _ storage.DB) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))

		entries, meta, err := indexer.ProcessBlockData(BlockData{Header: header, Events: []flow.Event{}})
		require.NoError(t, err)
		assert.Empty(t, entries)
		assert.Equal(t, 0, meta.Created)
		assert.Equal(t, 0, meta.Updated)
	})
}

// TestContractsIndexer_ProcessBlockData_ContractAdded verifies that ProcessBlockData returns
// the correct deployment entry and metadata for an AccountContractAdded event.
func TestContractsIndexer_ProcessBlockData_ContractAdded(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "MyContract"
	code := []byte("access(all) contract MyContract {}")
	codeHash := access.CadenceCodeHash(code)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, _ storage.LockManager, _ storage.DB) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		snap := makeContractSnapshot(t, address, map[string][]byte{contractName: code})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(snap, nil).Once()

		event := makeContractEvent(t, eventContractAdded, address, contractName, codeHash, 0, 0)

		entries, meta, err := indexer.ProcessBlockData(BlockData{Header: header, Events: []flow.Event{event}})
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, address, entries[0].Address)
		assert.Equal(t, contractName, entries[0].ContractName)
		assert.Equal(t, code, entries[0].Code)
		assert.Equal(t, 1, meta.Created)
		assert.Equal(t, 0, meta.Updated)
	})
}

// TestContractsIndexer_ProcessBlockData_ContractUpdated verifies that ProcessBlockData returns
// the correct metadata counts when processing an AccountContractUpdated event.
func TestContractsIndexer_ProcessBlockData_ContractUpdated(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	contractName := "MyContract"
	code := []byte("access(all) contract MyContract { let x: Int; init() { self.x = 2 } }")
	codeHash := access.CadenceCodeHash(code)

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, _ storage.LockManager, _ storage.DB) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		snap := makeContractSnapshot(t, address, map[string][]byte{contractName: code})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(snap, nil).Once()

		event := makeContractEvent(t, eventContractUpdated, address, contractName, codeHash, 0, 0)

		entries, meta, err := indexer.ProcessBlockData(BlockData{Header: header, Events: []flow.Event{event}})
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, 0, meta.Created)
		assert.Equal(t, 1, meta.Updated)
	})
}

// TestContractsIndexer_ProcessBlockData_DoesNotDependOnHeight verifies that ProcessBlockData
// can be called with any height without checking the indexer's height state.
func TestContractsIndexer_ProcessBlockData_DoesNotDependOnHeight(t *testing.T) {
	t.Parallel()

	runWithContractsIndexer(t, contractsBootstrapHeight, nil, nil, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, _ storage.LockManager, _ storage.DB) {
		// Use a height far beyond what the indexer expects
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight+100))

		entries, _, err := indexer.ProcessBlockData(BlockData{Header: header, Events: []flow.Event{}})
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

// TestContractsIndexer_ProcessBlockData_ContractRemoved verifies that ProcessBlockData returns
// an error when encountering a ContractRemoved event (same as IndexBlockData).
func TestContractsIndexer_ProcessBlockData_ContractRemoved(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	runWithContractsIndexer(t, contractsBootstrapHeight, nil, nil, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, _ storage.LockManager, _ storage.DB) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		removedEvent := makeContractEvent(t, eventContractRemoved, address, "SomeContract", make([]byte, 32), 0, 0)

		_, _, err := indexer.ProcessBlockData(BlockData{Header: header, Events: []flow.Event{removedEvent}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})
}

// ===== Error and edge-case tests =====

// TestContractsIndexer_AlreadyIndexed verifies that attempting to index the same height
// twice returns ErrAlreadyIndexed.
func TestContractsIndexer_AlreadyIndexed(t *testing.T) {
	t.Parallel()

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))

		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{}})
		require.NoError(t, err)

		err = indexContractsBlock(t, indexer, lm, db, BlockData{Header: header, Events: []flow.Event{}})
		require.ErrorIs(t, err, ErrAlreadyIndexed)
	})
}

// TestContractsIndexer_ContractRemoved verifies that an AccountContractRemoved event
// returns an error (contract removal is not supported). collectDeployments fails before
// loadDeployedContracts, so no script executor is needed.
func TestContractsIndexer_ContractRemoved(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	// collectDeployments errors for ContractRemoved before loadDeployedContracts runs,
	// so no GetStorageSnapshot call is made — use nil executor.
	runWithContractsIndexer(t, contractsBootstrapHeight, nil, nil, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		removedEvent := makeContractEvent(t, eventContractRemoved, address, "SomeContract", make([]byte, 32), 0, 0)

		err := indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: header,
			Events: []flow.Event{removedEvent},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})
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
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Bootstrap block.
		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})
		require.NoError(t, err)

		// Event block: snapshot has differentCode, but event hash is for eventCode.
		eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		snap := makeContractSnapshot(t, address, map[string][]byte{contractName: differentCode})
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(snap, nil).Once()

		event := makeContractEvent(t, eventContractAdded, address, contractName, eventHash, 0, 0)
		err = indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: eventHeader,
			Events: []flow.Event{event},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "code hash mismatch")
	})
}

// TestContractsIndexer_ScriptExecutorError verifies that an error from GetStorageSnapshot
// is propagated and causes IndexBlockData to fail.
func TestContractsIndexer_ScriptExecutorError(t *testing.T) {
	t.Parallel()

	address := unittest.RandomAddressFixture()
	scriptErr := fmt.Errorf("storage unavailable")

	scriptExecutor := executionmock.NewScriptExecutor(t)
	runWithContractsIndexer(t, contractsBootstrapHeight, &fakeRegisters{}, scriptExecutor, func(indexer *Contracts, _ storage.ContractDeploymentsIndexBootstrapper, lm storage.LockManager, db storage.DB) {
		// Bootstrap block.
		bootstrapHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsBootstrapHeight))
		err := indexContractsBlock(t, indexer, lm, db, BlockData{Header: bootstrapHeader, Events: []flow.Event{}})
		require.NoError(t, err)

		// Event block: GetStorageSnapshot fails.
		eventHeader := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(contractsEventHeight))
		scriptExecutor.On("GetStorageSnapshot", contractsEventHeight).Return(nil, scriptErr).Once()

		codeHash := access.CadenceCodeHash([]byte("access(all) contract MyContract {}"))
		event := makeContractEvent(t, eventContractAdded, address, "MyContract", codeHash, 0, 0)

		err = indexContractsBlock(t, indexer, lm, db, BlockData{
			Header: eventHeader,
			Events: []flow.Event{event},
		})
		require.Error(t, err)
		require.ErrorIs(t, err, scriptErr)
	})
}

// TestContractsIndexer_NextHeight_MockErrors verifies error propagation from the store.
func TestContractsIndexer_NextHeight_MockErrors(t *testing.T) {
	t.Parallel()

	t.Run("unexpected error from LatestIndexedHeight propagates", func(t *testing.T) {
		t.Parallel()

		mockStore := storagemock.NewContractDeploymentsIndexBootstrapper(t)
		unexpectedErr := fmt.Errorf("unexpected error")
		mockStore.On("LatestIndexedHeight").Return(uint64(0), unexpectedErr)

		indexer := NewContracts(unittest.Logger(), mockStore, nil, nil, &metrics.NoopCollector{})

		_, err := indexer.NextHeight()
		require.Error(t, err)
		require.ErrorIs(t, err, unexpectedErr)
	})

	t.Run("inconsistent state: not bootstrapped but initialized", func(t *testing.T) {
		t.Parallel()

		mockStore := storagemock.NewContractDeploymentsIndexBootstrapper(t)
		mockStore.On("LatestIndexedHeight").Return(uint64(0), storage.ErrNotBootstrapped)
		mockStore.On("UninitializedFirstHeight").Return(uint64(42), true)

		indexer := NewContracts(unittest.Logger(), mockStore, nil, nil, &metrics.NoopCollector{})

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
		indexer := NewContracts(unittest.Logger(), mockStore, nil, nil, &metrics.NoopCollector{})
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))

		err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
			return indexer.IndexBlockData(lctx, BlockData{Header: header, Events: []flow.Event{}}, nil)
		})
		require.Error(t, err)
		require.ErrorIs(t, err, storeErr)
	})
}

// ===== Test Setup Helpers =====

// fakeRegisters is a minimal registerScanner for tests. Populate Entries with register
// entries that should appear in the ByKeyPrefix scan (e.g. code registers for
// pre-existing contracts at the bootstrap height).
type fakeRegisters struct {
	Entries []flow.RegisterEntry
}

func (r *fakeRegisters) ByKeyPrefix(keyPrefix string, _ uint64, _ *flow.RegisterID) storage.IndexIterator[flow.RegisterValue, flow.RegisterID] {
	return func(yield func(storage.IteratorEntry[flow.RegisterValue, flow.RegisterID], error) bool) {
		for _, e := range r.Entries {
			if !strings.HasPrefix(e.Key.Key, keyPrefix) {
				continue
			}
			entry := fakeRegisterEntry{id: e.Key, val: e.Value}
			if !yield(entry, nil) {
				return
			}
		}
	}
}

type fakeRegisterEntry struct {
	id  flow.RegisterID
	val flow.RegisterValue
}

func (e fakeRegisterEntry) Cursor() flow.RegisterID            { return e.id }
func (e fakeRegisterEntry) Value() (flow.RegisterValue, error) { return e.val, nil }

// runWithContractsIndexer creates a temporary pebble DB and a Contracts indexer with the given
// registers and scriptExecutor, then calls f with the indexer, store, lock manager, and DB.
func runWithContractsIndexer(
	t *testing.T,
	firstHeight uint64,
	registers *fakeRegisters,
	scriptExecutor *executionmock.ScriptExecutor,
	f func(*Contracts, storage.ContractDeploymentsIndexBootstrapper, storage.LockManager, storage.DB),
) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		lm := storage.NewTestingLockManager()
		store, err := indexes.NewContractDeploymentsBootstrapper(db, firstHeight)
		require.NoError(t, err)
		indexer := NewContracts(unittest.Logger(), store, registers, scriptExecutor, &metrics.NoopCollector{})
		f(indexer, store, lm, db)
	})
}

// indexContractsBlock runs IndexBlockData with proper locking and batch commit and returns any error.
func indexContractsBlock(
	t *testing.T,
	indexer *Contracts,
	lm storage.LockManager,
	db storage.DB,
	data BlockData,
) error {
	t.Helper()
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

// encodeContractNames returns the CBOR-encoded contract names register value for the given names,
// matching the encoding used by accounts.setContractNames.
func encodeContractNames(t *testing.T, names ...string) flow.RegisterValue {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, cbor.NewEncoder(&buf).Encode(names))
	return buf.Bytes()
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

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType(eventTypeName),
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

func assertContractDeployment(t *testing.T, expected access.ContractDeployment, actual access.ContractDeployment) {
	assert.Equal(t, expected.Address, actual.Address)
	assert.Equal(t, expected.ContractName, actual.ContractName)
	assert.Equal(t, expected.BlockHeight, actual.BlockHeight)
	assert.Equal(t, expected.TransactionID, actual.TransactionID)
	assert.Equal(t, expected.TransactionIndex, actual.TransactionIndex)
	assert.Equal(t, expected.EventIndex, actual.EventIndex)
	assert.Equal(t, expected.Code, actual.Code)
	assert.Equal(t, expected.CodeHash, actual.CodeHash)
	assert.Equal(t, expected.IsPlaceholder, actual.IsPlaceholder)
	assert.Equal(t, expected.IsDeleted, actual.IsDeleted)
}
