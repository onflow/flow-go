package extended

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
	"github.com/onflow/flow-go/storage"
)

const (
	contractsIndexerName = "contracts"

	flowAccountContractAdded   flow.EventType = "flow.AccountContractAdded"
	flowAccountContractUpdated flow.EventType = "flow.AccountContractUpdated"
	flowAccountContractRemoved flow.EventType = "flow.AccountContractRemoved"

	// contractsBootstrapMaxIterationDuration is the maximum time the bootstrap scan holds a
	// pebble iterator open before releasing it and resuming from a cursor. Pebble defers
	// compaction while any iterator is open, so keeping iterations short lets compaction run
	// between batches and prevents read-amplification from building up during the potentially
	// hours-long initial scan.
	contractsBootstrapMaxIterationDuration = 30 * time.Second
)

// snapshotProvider is the subset of execution.ScriptExecutor used by the Contracts indexer.
type snapshotProvider interface {
	GetStorageSnapshot(height uint64) (snapshot.StorageSnapshot, error)
	RegisterValue(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

// registerScanner can efficiently scan all registers with a given key prefix in a single
// forward pass. Used during bootstrap to load all deployed contracts without iterating
// every account address.
type registerScanner interface {
	ByKeyPrefix(keyPrefix string, height uint64, cursor *flow.RegisterID) storage.IndexIterator[flow.RegisterValue, flow.RegisterID]
}

// Contracts indexes contract deployment lifecycle events and writes to the contract deployments
// index. Handles [flow.AccountContractAdded] and [flow.AccountContractUpdated] events.
// [flow.AccountContractRemoved] is not currently permitted; encountering one returns an error.
//
// On first bootstrapping, the indexer will load all deployed contracts from storage at the bootstrap
// height and include placeholder deployment objects for all contracts that are deployed at that height.
// Any contracts that are deployed in the same block as the bootstrap height will not have placeholder
// deployment objects created since there are already deployment objects for those contracts.
//
// CAUTION: Not safe for concurrent use.
type Contracts struct {
	log            zerolog.Logger
	metrics        module.ExtendedIndexingMetrics
	store          storage.ContractDeploymentsIndexBootstrapper
	registers      registerScanner
	scriptExecutor snapshotProvider
}

var _ Indexer = (*Contracts)(nil)

// NewContracts creates a new Contracts indexer backed by store.
func NewContracts(
	log zerolog.Logger,
	store storage.ContractDeploymentsIndexBootstrapper,
	registers registerScanner,
	scriptExecutor snapshotProvider,
	metrics module.ExtendedIndexingMetrics,
) *Contracts {
	return &Contracts{
		log:            log.With().Str("component", "contracts_indexer").Logger(),
		metrics:        metrics,
		store:          store,
		registers:      registers,
		scriptExecutor: scriptExecutor,
	}
}

// Name returns the name of this indexer.
func (c *Contracts) Name() string { return contractsIndexerName }

// NextHeight returns the next block height to index.
//
// No error returns are expected during normal operation.
func (c *Contracts) NextHeight() (uint64, error) {
	return nextHeight(c.store)
}

// IndexBlockData processes one block's events and transactions and updates the contract
// deployments index.
//
// The caller must hold the [storage.LockIndexContractDeployments] lock until the batch is committed.
//
// CAUTION: Not safe for concurrent use.
//
// Expected error returns during normal operations:
//   - [ErrAlreadyIndexed]: if the data is already indexed for the height
func (c *Contracts) IndexBlockData(lctx lockctx.Proof, data BlockData, rw storage.ReaderBatchWriter) error {
	deployments, created, updated, err := c.collectDeployments(data)
	if err != nil {
		return fmt.Errorf("failed to collect contract deployments for block %s: %w", data.Header.ID(), err)
	}

	// if storage is not bootstrapped yet, do an initial load of all deployed contracts and include it in
	// the initial store operation.
	// TODO: avoid doing this check on every block. in the mean time, this is a fast check
	if bootstrapHeight, isInitialized := c.store.UninitializedFirstHeight(); !isInitialized {
		start := time.Now()

		// keep track of the contracts with deployments in this block. these can be skipped during the
		// initial load since the deployment object is already created.
		updatedContracts := make(map[string]bool, len(deployments))
		for _, deployment := range deployments {
			updatedContracts[deployment.ContractID] = true
		}

		// TODO: this will currently load all contracts into memory, then store them in one large batch.
		// this is probably ok for now since there is a relatively small number of contracts. This should
		// be updated to allow storing the contracts in smaller batches.

		bootstrapContracts, err := c.loadDeployedContracts(bootstrapHeight, updatedContracts)
		if err != nil {
			return fmt.Errorf("failed to load deployed contracts: %w", err)
		}
		deployments = append(deployments, bootstrapContracts...)

		c.log.Info().
			Uint64("height", bootstrapHeight).
			Int("contracts", len(bootstrapContracts)).
			Dur("dur_ms", time.Since(start)).
			Msg("loaded deployed contracts during bootstrap")
	}

	if err := c.store.Store(lctx, rw, data.Header.Height, deployments); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			return ErrAlreadyIndexed
		}
		return fmt.Errorf("failed to store contract deployments for block %s: %w", data.Header.ID(), err)
	}

	c.metrics.ContractDeploymentIndexed(created, updated) // ignores bootstrap deployments
	return nil
}

// collectDeployments iterates the block events and builds a [access.ContractDeployment] for
// each AccountContractAdded or AccountContractUpdated event found. Returns the deployments
// and the counts of created and updated contracts.
//
// No error returns are expected during normal operation.
func (c *Contracts) collectDeployments(data BlockData) (deployments []access.ContractDeployment, created, updated int, err error) {
	retriever := newContractRetriever(c.scriptExecutor, data.Header.Height)

	for _, event := range data.Events {
		switch event.Type {
		case flowAccountContractAdded:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event payload: %w", event.Type, err)
			}
			e, err := events.DecodeAccountContractAdded(cadenceEvent)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event: %w", event.Type, err)
			}

			// script executor gets data at the end of the block, so code here should be the updated version
			code, err := retriever.contractCode(e.Address, e.ContractName, data.Header.Height)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to get contract code: %w", err)
			}

			// make sure the hash of the code fetched from state matches the hash in the event
			if !bytes.Equal(e.CodeHash, access.CadenceCodeHash(code)) {
				return nil, 0, 0, fmt.Errorf("code hash mismatch for %s event: %s", event.Type, e.ContractName)
			}

			deployments = append(deployments, access.ContractDeployment{
				ContractID:       events.ContractIDFromAddress(e.Address, e.ContractName),
				Address:          e.Address,
				BlockHeight:      data.Header.Height,
				TransactionID:    event.TransactionID,
				TransactionIndex: event.TransactionIndex,
				EventIndex:       event.EventIndex,
				Code:             code,
				CodeHash:         e.CodeHash,
			})
			created++

		case flowAccountContractUpdated:
			cadenceEvent, err := events.DecodePayload(event)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event payload: %w", event.Type, err)
			}
			e, err := events.DecodeAccountContractUpdated(cadenceEvent)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to decode %s event: %w", event.Type, err)
			}

			// script executor gets data at the end of the block, so code here should be the updated version
			code, err := retriever.contractCode(e.Address, e.ContractName, data.Header.Height)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("failed to get account code: %w", err)
			}

			// make sure the hash of the code fetched from state matches the hash in the event
			if !bytes.Equal(e.CodeHash, access.CadenceCodeHash(code)) {
				return nil, 0, 0, fmt.Errorf("code hash mismatch for %s event: %s", event.Type, e.ContractName)
			}

			deployments = append(deployments, access.ContractDeployment{
				ContractID:       events.ContractIDFromAddress(e.Address, e.ContractName),
				Address:          e.Address,
				BlockHeight:      data.Header.Height,
				TransactionID:    event.TransactionID,
				TransactionIndex: event.TransactionIndex,
				EventIndex:       event.EventIndex,
				Code:             code,
				CodeHash:         e.CodeHash,
			})
			updated++

		case flowAccountContractRemoved:
			// contract removal is not currently supported. returning an error here will cause the
			// indexer to crash, signalling that implementation is needed.
			return nil, 0, 0, fmt.Errorf("unexpected %s event in block %s tx %s: not supported",
				event.Type, data.Header.ID(), event.TransactionID)
		}
	}

	return deployments, created, updated, nil
}

// loadDeployedContracts scans all code registers at the given height and returns a
// access.ContractDeployment placeholder record for each deployed contract not already
// covered by seenContracts.
//
// The scan is broken into iterations bounded by [contractsBootstrapMaxIterationDuration].
// Each iteration holds a single pebble iterator open; releasing it between iterations
// allows pebble compaction to run and prevents read-amplification build-up during the
// potentially hours-long bootstrap scan.
//
// No error returns are expected during normal operation.
func (c *Contracts) loadDeployedContracts(height uint64, seenContracts map[string]bool) ([]access.ContractDeployment, error) {
	var deployments []access.ContractDeployment
	var cursor *flow.RegisterID
	skippedAlreadySeen := 0
	loadedContracts := make(map[flow.Address][]string)
	for {
		batchStart := time.Now()
		timedOut := false

		for entry, err := range c.registers.ByKeyPrefix(flow.CodeKeyPrefix, height, cursor) {
			if err != nil {
				return nil, fmt.Errorf("error scanning contract code registers: %w", err)
			}

			reg := entry.Cursor()
			address := flow.BytesToAddress([]byte(reg.Owner))
			contractName := flow.KeyContractName(reg.Key)
			contractID := events.ContractIDFromAddress(address, contractName)

			if !seenContracts[contractID] {
				// when a contract is deleted, it's code is set to nil/empty and the name is removed
				// from the contract names register. the actual register is still present in state.
				// while deleting contracts is not supported, there are deleted contracts in the state.
				code, err := entry.Value()
				if err != nil {
					return nil, fmt.Errorf("error reading contract code for %s: %w", contractID, err)
				}
				if len(code) == 0 {
					continue // skip deleted contracts
				}

				deployments = append(deployments, access.ContractDeployment{
					ContractID: contractID,
					Address:    address,
					Code:       code,
					CodeHash:   access.CadenceCodeHash(code),
					// all other fields are omitted because we do not know the actual deployment details
					IsPlaceholder: true,
				})
				c.log.Info().
					Str("contract_id", contractID).
					Msg("loaded contract during bootstrap")
			} else {
				skippedAlreadySeen++
			}

			loadedContracts[address] = append(loadedContracts[address], contractName)

			// If we've held the iterator open too long, record the cursor and release it so
			// pebble compaction can proceed before we resume.
			if time.Since(batchStart) >= contractsBootstrapMaxIterationDuration {
				cursor = &reg
				timedOut = true
				break
			}
		}

		if !timedOut {
			break
		}
	}

	c.log.Info().
		Uint64("height", height).
		Int("contracts", len(deployments)).
		Int("skipped_already_seen", skippedAlreadySeen).
		Msg("loaded contracts during bootstrap")

	// verify that the contract names in the contract names register match the contract names in the loaded contracts
	for address, loadedContractNames := range loadedContracts {
		registerValue, err := c.scriptExecutor.RegisterValue(flow.ContractNamesRegisterID(address), height)
		if err != nil {
			return nil, fmt.Errorf("error getting contract names for %s: %w", address, err)
		}

		contractNames, err := environment.DecodeContractNames(registerValue)
		if err != nil {
			return nil, fmt.Errorf("error decoding contract names for %s: %w", address, err)
		}

		sort.Strings(contractNames)
		sort.Strings(loadedContractNames)

		if len(contractNames) != len(loadedContractNames) {
			return nil, fmt.Errorf("contract names length mismatch for %s: %d != %d", address, len(contractNames), len(loadedContractNames))
		}

		for i := range contractNames {
			if contractNames[i] != loadedContractNames[i] {
				return nil, fmt.Errorf("contract name mismatch for %s: %s != %s", address, contractNames[i], loadedContractNames[i])
			}
		}
	}

	return deployments, nil
}

// contractRetriever is a helper for retrieving contract code from the snapshot at the given height.
// It lazily sets up the accounts instance, so unused instances are cheap to create.
//
// CAUTION: Not safe for concurrent use.
type contractRetriever struct {
	height         uint64
	accounts       environment.Accounts
	scriptExecutor snapshotProvider
}

func newContractRetriever(scriptExecutor snapshotProvider, height uint64) *contractRetriever {
	return &contractRetriever{
		height:         height,
		scriptExecutor: scriptExecutor,
	}
}

// contractCode returns the code for the given contract at the given height.
//
// CAUTION: Not safe for concurrent use.
//
// No error returns are expected during normal operation.
func (c *contractRetriever) contractCode(
	address flow.Address,
	contractName string,
	height uint64,
) ([]byte, error) {
	if c.height != height {
		return nil, fmt.Errorf("height mismatch: %d != %d", c.height, height)
	}

	// lazily setup accounts since many blocks will not have any contract updates.
	if c.accounts == nil {
		snapshot, err := c.scriptExecutor.GetStorageSnapshot(height)
		if err != nil {
			return nil, fmt.Errorf("failed to get storage snapshot: %w", err)
		}

		txnState := state.NewTransactionState(snapshot, state.DefaultParameters())
		c.accounts = environment.NewAccounts(txnState)
	}

	code, err := c.accounts.GetContract(contractName, address)
	if err != nil {
		return nil, fmt.Errorf("error while getting contract: %w", err)
	}

	return code, nil
}
