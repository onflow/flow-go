package inspection

import (
	"fmt"
	"math"
	"runtime/debug"
	"sync"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/bbq/vm"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/systemcontracts"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type TokenChanges struct {
	// searchedTokens holds a reference to the map of tokens to search
	// its shallow copied from whatever is specified from the outside, so that the locking
	// should work properly
	searchedTokens   TokenChangesSearchTokens
	searchedTokensMu sync.RWMutex
}

var _ Inspector = (*TokenChanges)(nil)

// NewTokenChangesInspector return a TokenChanges inspector, that will be run
// after transaction execution and analyze if any unaccounted tokens were created or
// destroy.
func NewTokenChangesInspector(searchedTokens TokenChangesSearchTokens) *TokenChanges {
	return &TokenChanges{searchedTokens: searchedTokens}
}

// SetSearchedTokens are safe to replace whenever.
// The change will not affect the inspections already in progress.
// TODO: this can be tied into the admin commands
func (td *TokenChanges) SetSearchedTokens(searchedTokens TokenChangesSearchTokens) {
	// copy the map in case the user tries to modify the map
	st := make(map[string]SearchToken, len(searchedTokens))
	for k, v := range searchedTokens {
		st[k] = v
	}
	td.searchedTokensMu.Lock()
	defer td.searchedTokensMu.Unlock()
	td.searchedTokens = st
}

func (td *TokenChanges) getSearchedTokensRef() TokenChangesSearchTokens {
	td.searchedTokensMu.RLock()
	defer td.searchedTokensMu.RUnlock()
	return td.searchedTokens
}

// Inspect gets the token diff from a state diff
// - thread safe
// - not deterministic (iterates over maps)! So it should not be used to affect execution!
// - will not panic
// - might return an error, but it is safe to ignore since this for information/reporting
//
// Inspect could technically be run on chunk data packs.
func (td *TokenChanges) Inspect(
	logger zerolog.Logger,
	storage snapshot.StorageSnapshot,
	executionSnapshot *snapshot.ExecutionSnapshot,
	events []flow.Event,
) (diff Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn().Str("module", "tc-inspector").Msgf("trace=%s", string(debug.Stack()))
			err = fmt.Errorf("panic: %v", r)
		}

		if err != nil {
			err = fmt.Errorf("failed to get token diff: %w", err)
		}
	}()

	diff, err = td.getTokenDiff(logger, storage, executionSnapshot, events, td.getSearchedTokensRef())
	return
}

func (td *TokenChanges) getTokenDiff(
	logger zerolog.Logger,
	storage snapshot.StorageSnapshot,
	executionSnapshot *snapshot.ExecutionSnapshot,
	events []flow.Event,
	searchedTokens map[string]SearchToken,
) (TokenDiffResult, error) {
	executionSnapshotLedgers := executionSnapshotLedgers{
		StorageSnapshot:   storage,
		ExecutionSnapshot: executionSnapshot,
	}

	// get all distinct addresses
	addresses := make(map[common.Address]struct{})
	for k := range executionSnapshotLedgers.allTouchedRegisters() {
		// skip special registers
		// none of them can hold resources
		if len(k.Owner) == 0 {
			continue
		}
		addresses[common.Address([]byte(k.Owner))] = struct{}{}
	}

	oldRegistersLedger := executionSnapshotLedgers.OldValuesLedger()
	newValuesRegister := executionSnapshotLedgers.NewValuesLedger()

	// TODO: possible optimisation: run both at the same time
	before, err := td.getTokens(logger, oldRegistersLedger, addresses, tokenDiffSearchDomains, searchedTokens)
	if err != nil {
		return TokenDiffResult{}, fmt.Errorf("failed to get tokens before: %w", err)
	}
	after, err := td.getTokens(logger, newValuesRegister, addresses, tokenDiffSearchDomains, searchedTokens)
	if err != nil {
		return TokenDiffResult{}, fmt.Errorf("failed to get tokens after: %w", err)
	}

	typicalDiffSize := 4 // from, to, payer and fees
	tokenDiffResult := TokenDiffResult{
		Changes: make(map[flow.Address]AccountChange, typicalDiffSize),
	}

	for a := range addresses {
		beforeTokens := fmt.Sprintf("%v", before[a])
		afterTokens := fmt.Sprintf("%v", after[a])
		diff := diffAccountTokens(before[a], after[a])
		if len(diff) == 0 {
			logger.Info().Str("module", "tc-inspector").Msgf("account token change: %s is the same: before=%s after=%s", a, beforeTokens, afterTokens)
			continue
		} else {
			logger.Info().Str("module", "tc-inspector").Msgf("account token change: %s changed: before=%s after=%s diff=%v", a, beforeTokens, afterTokens, diff)
		}
		tokenDiffResult.Changes[flow.Address(a)] = diff
	}

	sourcesSinks, err := td.findSourcesSinks(events, searchedTokens)
	if err != nil {
		return TokenDiffResult{}, fmt.Errorf("failed to find sources/sinks: %w", err)
	}
	tokenDiffResult.KnownSourcesSinks = sourcesSinks

	return tokenDiffResult, nil
}

func (td *TokenChanges) getTokens(
	logger zerolog.Logger,
	storage ledgerSnapshot,
	addresses map[common.Address]struct{},
	domains []common.StorageDomain,
	searchedTokens map[string]SearchToken,
) (map[common.Address]accountTokens, error) {
	storageConfig := runtime.StorageConfig{}
	runtimeStorage := runtime.NewStorage(storage, nil, nil, storageConfig)

	// without this the tokens are not properly detected!
	// TODO: choose a good number for the workers
	err := loadAtreeSlabsInStorage(runtimeStorage, storage, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to load atree slabs: %w", err)
	}

	storageRuntime, err := newReadonlyStorageRuntimeWithStorage(runtimeStorage, runtimeStorage.Count())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage runtime: %w", err)
	}

	tokens := make(map[common.Address]accountTokens, len(addresses))
	for a := range addresses {
		tkns := make(accountTokens, len(searchedTokens))
		for _, d := range domains {
			// We are making the assumption that if a register was changed, the registers read to make that change
			// are enough to read that register before the change (if it existed)

			// It seems that GetDomainStorageMap() tries to load domain storage map if it isn't loaded.
			// This causes a "slab not found" panic because we only included loaded registers in the underlying storage.
			// This workaround is to catch the panic gracefully so we can continue to inspect the next domain storage map.
			storageMap := getDomainStorageMap(logger, storageRuntime, storageRuntime.Interpreter, a, d)
			if storageMap == nil {
				continue
			}

			iter := storageMap.ReadOnlyLoadedValueIterator()
			for {
				interpreterValue := iter.NextValue(nil)

				if interpreterValue == nil {
					break
				}

				walkLoaded(interpreterValue, searchedTokens, tkns)
			}
		}
		if len(tkns) > 0 {
			logger.Debug().Str("module", "tc-inspector").Msgf("found tokens for %s: %v", a, tkns)
		}
		tokens[a] = tkns
	}
	return tokens, nil
}

func getDomainStorageMap(
	logger zerolog.Logger,
	storageRuntime *readonlyStorageRuntime,
	storageMutationTracker interpreter.StorageMutationTracker,
	address common.Address,
	domain common.StorageDomain,
) (dm *interpreter.DomainStorageMap) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn().Str("module", "tc-inspector").Msgf("failed to get domain storage map %s.%s: %v", address.String(), domain.Identifier(), r)
			dm = nil
		}
	}()
	return storageRuntime.Storage.GetDomainStorageMap(storageMutationTracker, address, domain, false)
}

func walkLoaded(
	value interpreter.Value,
	searchedTokens map[string]SearchToken,
	tkns accountTokens,
) {
	// The context is not needed for the walk,
	// but a context of nil produces an error.
	c := &vm.Context{
		Config: &vm.Config{},
	}

	var f func(value interpreter.Value)
	f = func(value interpreter.Value) {
		switch v := value.(type) {
		case *interpreter.CompositeValue:
			t, ok := searchedTokens[string(v.TypeID())]
			if ok {
				tkns.add(t.ID, t.GetBalance(v))
			}

			// technically nothing is stopping you from putting a vault into a vault, so we have to continue walking
			v.ForEachReadOnlyLoadedField(c, func(fieldName string, fieldValue interpreter.Value) (resume bool) {
				f(fieldValue)
				return true
			})
		case *interpreter.DictionaryValue:
			v.IterateReadOnlyLoaded(c, func(key interpreter.Value, value interpreter.Value) (resume bool) {
				f(key)
				f(value)
				return true
			})
		case *interpreter.ArrayValue:
			v.IterateReadOnlyLoaded(c, func(value interpreter.Value) (resume bool) {
				f(value)
				return true
			})
		default:
			// This assumes all other types cannot be partially loaded.
			v.Walk(c, f)
		}
	}
	f(value)
}

func (td *TokenChanges) findSourcesSinks(events []flow.Event, tokens map[string]SearchToken) (map[string]int64, error) {
	// create a map of all sinks and sources
	// TODO: could be created once
	type tokenSourceSink struct {
		tokenID string
		f       func(flow.Event) (int64, error)
	}
	sourcesSinks := make(map[string]tokenSourceSink)
	results := make(map[string]int64)
	for _, token := range tokens {
		for evt, ss := range token.SinksSources {
			sourcesSinks[evt] = tokenSourceSink{tokenID: token.ID, f: ss}
		}
	}

	for _, evt := range events {
		id := string(evt.Type)
		if ss, ok := sourcesSinks[id]; ok {
			v, err := ss.f(evt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse source/sink event %s: %w", id, err)
			}
			results[ss.tokenID] += v
		}
	}

	return results, nil
}

func newReadonlyStorageRuntimeWithStorage(storage *runtime.Storage, payloadCount int) (*readonlyStorageRuntime, error) {
	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		&interpreter.Config{
			Storage: storage,
		},
	)
	if err != nil {
		return nil, err
	}

	return &readonlyStorageRuntime{
		Interpreter:  inter,
		Storage:      storage,
		PayloadCount: payloadCount,
	}, nil
}

type executionSnapshotLedgers struct {
	snapshot.StorageSnapshot
	*snapshot.ExecutionSnapshot
}

func (l executionSnapshotLedgers) SetValue(owner, key, value []byte) (err error) {
	panic("unexpected call of SetValue.")
}

func (l executionSnapshotLedgers) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	panic("unexpected call of AllocateSlabIndex")
}

type executionSnapshotLedgersOld struct {
	executionSnapshotLedgers
}

func (o executionSnapshotLedgersOld) Get(owner string, key string) ([]byte, error) {
	return o.GetValue([]byte(owner), []byte(key))
}

func (o executionSnapshotLedgersOld) Set(owner string, key string, value []byte) error {
	return o.SetValue([]byte(owner), []byte(key), value)
}

func (o executionSnapshotLedgersOld) ForEach(f forEachCallback) error {
	for key := range o.ExecutionSnapshot.ReadSet {
		id := flow.NewRegisterID(flow.BytesToAddress([]byte(key.Owner)), key.Key)

		v, err := o.StorageSnapshot.Get(id)
		if err != nil {
			return err
		}

		err = f(key.Owner, key.Key, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o executionSnapshotLedgersOld) Count() int {
	return len(o.ExecutionSnapshot.ReadSet)
}

func (n executionSnapshotLedgersNew) Get(owner string, key string) ([]byte, error) {
	return n.GetValue([]byte(owner), []byte(key))
}

func (n executionSnapshotLedgersNew) Set(owner string, key string, value []byte) error {
	return n.SetValue([]byte(owner), []byte(key), value)
}

func (n executionSnapshotLedgersNew) ForEach(f forEachCallback) error {
	for key := range n.allTouchedRegisters() {
		v, err := n.GetValue([]byte(key.Owner), []byte(key.Key))
		if err != nil {
			return err
		}

		err = f(key.Owner, key.Key, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (n executionSnapshotLedgersNew) Count() int {
	return len(n.allTouchedRegisters())
}

// allTouchedRegisters returns both read and written to registers.
func (l executionSnapshotLedgers) allTouchedRegisters() map[flow.RegisterID]struct{} {
	fullSet := make(map[flow.RegisterID]struct{})
	for key := range l.ExecutionSnapshot.ReadSet {
		fullSet[flow.NewRegisterID(flow.BytesToAddress([]byte(key.Owner)), key.Key)] = struct{}{}
	}
	for key := range l.ExecutionSnapshot.WriteSet {
		fullSet[flow.NewRegisterID(flow.BytesToAddress([]byte(key.Owner)), key.Key)] = struct{}{}
	}
	return fullSet
}

type ledgerSnapshot interface {
	atree.Ledger
	registers
}

type executionSnapshotLedgersNew struct {
	executionSnapshotLedgers
}

func (l executionSnapshotLedgers) OldValuesLedger() ledgerSnapshot {
	return executionSnapshotLedgersOld{l}
}

func (l executionSnapshotLedgers) NewValuesLedger() ledgerSnapshot {
	return executionSnapshotLedgersNew{l}
}

var _ registers = &executionSnapshotLedgersOld{}

func (o executionSnapshotLedgersOld) GetValue(owner, key []byte) (value []byte, err error) {
	id := flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
	_, ok := o.ExecutionSnapshot.ReadSet[id]
	if !ok {
		return nil, nil
	}

	v, err := o.StorageSnapshot.Get(id)
	return v, err
}

func (o executionSnapshotLedgersOld) ValueExists(owner, key []byte) (exists bool, err error) {
	v, err := o.GetValue(owner, key)
	return len(v) > 0, err
}

func (n executionSnapshotLedgersNew) GetValue(owner, key []byte) (value []byte, err error) {
	id := flow.NewRegisterID(flow.BytesToAddress(owner), string(key))

	v, ok := n.ExecutionSnapshot.WriteSet[id]

	if ok {
		return v, nil
	}

	_, ok = n.ExecutionSnapshot.ReadSet[id]
	if !ok {
		return nil, nil
	}

	v, err = n.StorageSnapshot.Get(id)
	return v, err
}

func (n executionSnapshotLedgersNew) ValueExists(owner, key []byte) (exists bool, err error) {
	v, err := n.GetValue(owner, key)
	return len(v) > 0, err
}

type readonlyStorageRuntime struct {
	Interpreter  *interpreter.Interpreter
	Storage      *runtime.Storage
	PayloadCount int
}

type SearchToken struct {
	ID         string
	GetBalance func(value *interpreter.CompositeValue) uint64
	// TODO: optimize by using decoded events
	SinksSources map[string]func(flow.Event) (int64, error)
}

// TokenDiffResult is the result of the inspection
type TokenDiffResult struct {
	// Changes in token balances per account
	// parsed from the state changes
	Changes map[flow.Address]AccountChange

	// KnownSourcesSinks is a map (by token id) of
	// know mints/burns for the token parsed from predetermined events
	KnownSourcesSinks map[string]int64
}

var _ Result = TokenDiffResult{}

func (r TokenDiffResult) AsLogEvent() (zerolog.Level, func(e *zerolog.Event)) {
	sum := r.UnaccountedTokens()
	if len(sum) == 0 {
		// everything is ok: log no issues with debug logging
		return zerolog.DebugLevel, func(e *zerolog.Event) { e.Str("token_diff", "no issues") }
	}

	anyPositive := false
	for _, v := range sum {
		if v > 0 {
			anyPositive = true
			break
		}
	}

	level := zerolog.WarnLevel
	if anyPositive {
		// if any tracked token increase in supply
		// log at error level
		level = zerolog.ErrorLevel
	}

	return level, func(e *zerolog.Event) {
		dict := zerolog.Dict()
		for k, v := range sum {
			dict = dict.Int64(k, v)
		}
		e.Dict("token_diff", dict)
	}
}

func (r TokenDiffResult) UnaccountedTokens() map[string]int64 {
	sum := make(map[string]int64)
	for _, change := range r.Changes {
		for token, amount := range change {
			sum[token] += amount
		}
	}

	for k, v := range r.KnownSourcesSinks {
		// Yes this should be -.
		// If we mint 100 tokens, that will be a source of +100 and the sum will contain
		// the +100 we minted. We need to account for that +100 in the sum by **subtracting**
		// the +100 we expected from the mint event.
		sum[k] -= v
	}

	// delete empty entries
	for k, v := range sum {
		if v == 0 {
			delete(sum, k)
		}
	}
	return sum
}

type AccountChange map[string]int64

type accountTokens map[string]uint64

func (t accountTokens) add(id string, value uint64) {
	t[id] += value
}

// diffAccountTokens will potentially modify before and after, so they should not be reused
// after this call.
func diffAccountTokens(before accountTokens, after accountTokens) AccountChange {
	change := make(AccountChange)

	for k, a := range after {
		if b, ok := before[k]; ok {
			if a > b {
				change[k] = safeUint64ToInt64(a - b)
			} else if b > a {
				change[k] = -safeUint64ToInt64(b - a)
			}
			delete(before, k)
		} else {
			change[k] = safeUint64ToInt64(a)
		}
	}

	for k, v := range before {
		change[k] = -safeUint64ToInt64(v)
	}

	return change
}

// safeUint64ToInt64 converts a uint64 to int64, capping at math.MaxInt64 if the value
// would overflow. If a value is capped, it will cause a mismatch in the token accounting
// which will be logged and raise attention anyway.
func safeUint64ToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

type forEachCallback func(owner string, key string, value []byte) error

type registers interface {
	Get(owner string, key string) ([]byte, error)
	Set(owner string, key string, value []byte) error
	ForEach(f forEachCallback) error
	Count() int
}

func loadAtreeSlabsInStorage(
	storage *runtime.Storage,
	registers registers,
	nWorkers int,
) error {

	storageIDs, err := getSlabIDsFromRegisters(registers)
	if err != nil {
		return err
	}

	return storage.PersistentSlabStorage.BatchPreload(storageIDs, nWorkers)
}

func getSlabIDsFromRegisters(registers registers) ([]atree.SlabID, error) {
	storageIDs := make([]atree.SlabID, 0, registers.Count())

	err := registers.ForEach(func(owner string, key string, _ []byte) error {
		if !flow.IsSlabIndexKey(key) {
			return nil
		}

		slabID := atree.NewSlabID(
			atree.Address([]byte(owner)),
			atree.SlabIndex([]byte(key[1:])),
		)

		storageIDs = append(storageIDs, slabID)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return storageIDs, nil
}

var tokenDiffSearchDomains = []common.StorageDomain{
	common.StorageDomainPathStorage,
	common.StorageDomainContract,

	common.StorageDomainPathPrivate, // ?? probably not needed. TODO: check
	common.StorageDomainInbox,       // ?? probably not needed. TODO: check
	// do we need any other? TODO: check
}

type TokenChangesSearchTokens map[string]SearchToken

func DefaultTokenDiffSearchTokens(chain flow.Chain) TokenChangesSearchTokens {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	flowTokenID := fmt.Sprintf("A.%s.FlowToken.Vault", sc.FlowToken.Address.Hex())
	flowTokenMintedEventID := fmt.Sprintf("A.%s.FlowToken.TokensMinted", sc.FlowToken.Address.Hex())

	return map[string]SearchToken{
		flowTokenID: {
			ID: flowTokenID,
			GetBalance: func(value *interpreter.CompositeValue) uint64 {
				return uint64(value.GetField(nil, "balance").(interpreter.UFix64Value).UFix64Value)
			},
			SinksSources: map[string]func(flow.Event) (int64, error){
				flowTokenMintedEventID: func(evt flow.Event) (int64, error) {
					// this decoding will only happen for the specified event (in the case of FlowToken.TokensMinted it
					// is extremely rare).
					payload, err := ccf.Decode(nil, evt.Payload)
					if err != nil {
						return 0, err
					}
					v := payload.(cadence.Event).SearchFieldByName("amount")
					if v == nil {
						return 0, fmt.Errorf("no amount field found for token minted")
					}

					ufix, ok := payload.(cadence.Event).SearchFieldByName("amount").(cadence.UFix64)
					if !ok {
						return 0, fmt.Errorf("amount field is not a cadence.UFix64")
					}

					if ufix > math.MaxInt64 {
						// this is very unlikely
						// but in case it happens, it will get logged
						return 0, fmt.Errorf("amount field is too large")
					}

					return int64(ufix), nil
				},
			},
		},
	}
}
