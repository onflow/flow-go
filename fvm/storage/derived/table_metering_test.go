package derived

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/state"
)

// meteringValueComputer meters a fixed computation usage when computing the
// derived value, mimicking the metering performed while loading a program
// (e.g. get_code, resolve_location, get_account_contract_code).
type meteringValueComputer struct {
	usage common.ComputationUsage
}

func (c meteringValueComputer) Compute(
	txnState state.NestedTransactionPreparer,
	_ string,
) (int, error) {
	return 0, txnState.MeterComputation(c.usage)
}

// nestedValueComputer meters its own usage and then loads an inner value from
// another table via the same transaction, mimicking a program importing
// another program: the inner load's charges must be captured in the outer
// entry's cached snapshot.
type nestedValueComputer struct {
	usage         common.ComputationUsage
	innerTable    *TableTransaction[string, int]
	innerComputer ValueComputer[string, int]
}

func (c nestedValueComputer) Compute(
	txnState state.NestedTransactionPreparer,
	_ string,
) (int, error) {
	if err := txnState.MeterComputation(c.usage); err != nil {
		return 0, err
	}
	_, err := c.innerTable.GetOrCompute(txnState, "inner", c.innerComputer)
	return 0, err
}

// TestGetOrComputeMeteringWithNestedLoad checks that a load which itself loads
// a nested value (import) charges the same warm or cold: the inner load's
// computation is captured in the outer entry's cached snapshot and replayed on
// a hit (see internal issue #7126).
func TestGetOrComputeMeteringWithNestedLoad(t *testing.T) {
	outer := common.ComputationUsage{Kind: common.ComputationKindStatement, Intensity: 5}
	inner := common.ComputationUsage{Kind: common.ComputationKindStatement, Intensity: 2}

	// charge loads the outer key (which loads the inner key) in a fresh
	// transaction and returns the total metered intensity.
	charge := func(t *testing.T, outerTable, innerTable *TableTransaction[string, int]) uint64 {
		computer := nestedValueComputer{
			usage:         outer,
			innerTable:    innerTable,
			innerComputer: meteringValueComputer{usage: inner},
		}
		txnState := state.NewTransactionState(nil, state.DefaultParameters())
		_, err := outerTable.GetOrCompute(txnState, "outer", computer)
		require.NoError(t, err)
		return txnState.ComputationIntensities()[common.ComputationKindStatement]
	}

	// Cold cache: both values are computed by the reading transaction.
	coldOuter := NewEmptyTable[string, int](0)
	coldInner := NewEmptyTable[string, int](0)
	coldOuterTxn, err := coldOuter.NewTableTransaction(0, 0)
	require.NoError(t, err)
	coldInnerTxn, err := coldInner.NewTableTransaction(0, 0)
	require.NoError(t, err)
	cold := charge(t, coldOuterTxn, coldInnerTxn)

	// Warm cache: an earlier transaction populated both tables.
	warmOuter := NewEmptyTable[string, int](0)
	warmInner := NewEmptyTable[string, int](0)
	loadOuterTxn, err := warmOuter.NewTableTransaction(0, 0)
	require.NoError(t, err)
	loadInnerTxn, err := warmInner.NewTableTransaction(0, 0)
	require.NoError(t, err)
	_ = charge(t, loadOuterTxn, loadInnerTxn)
	require.NoError(t, loadInnerTxn.Commit())
	require.NoError(t, loadOuterTxn.Commit())

	hitOuterTxn, err := warmOuter.NewTableTransaction(1, 1)
	require.NoError(t, err)
	hitInnerTxn, err := warmInner.NewTableTransaction(1, 1)
	require.NoError(t, err)
	warm := charge(t, hitOuterTxn, hitInnerTxn)

	require.Equal(t, outer.Intensity+inner.Intensity, cold)
	require.Equal(t, cold, warm, "nested load must charge the same warm or cold")
}

// TestGetOrComputeMeteringIsIndependentOfCacheState covers
// https://github.com/onflow/flow-go-internal/issues/7126.
//
// The computation charged to a transaction by GetOrCompute (used by the
// programs cache) must be identical whether the value is computed (cache
// miss) or replayed from the cache (cache hit). Otherwise metering is
// non-deterministic: execution nodes with warm and cold caches charge
// different amounts for the same transaction.
//
// The test exercises all four combinations of the metering scope in which the
// value is first loaded into the cache, and the metering scope in which it is
// subsequently read. It also asserts the absolute charge: a metering-enabled
// read is fully charged and a metering-disabled read is not charged at all,
// regardless of cache state.
//
// Historical note: before the fix, load(true)/read(false) phantom-charged the
// cached snapshot's meter (ExecutionState.Merge merged meters
// unconditionally), and load(false)/read(true) replayed zero charges (the
// loading nested transaction shared the caller's disabled limitsController,
// poisoning the cache with an empty meter).
func TestGetOrComputeMeteringIsIndependentOfCacheState(t *testing.T) {
	const key = "key"
	usage := common.ComputationUsage{
		Kind:      common.ComputationKindStatement,
		Intensity: 7,
	}
	computer := meteringValueComputer{usage: usage}

	// charge runs GetOrCompute in a fresh transaction state within the given
	// metering scope, and returns the intensity recorded on the transaction's
	// meter for usage.Kind.
	charge := func(
		t *testing.T,
		tableTxn *TableTransaction[string, int],
		meteringEnabled bool,
	) uint64 {
		txnState := state.NewTransactionState(nil, state.DefaultParameters())
		run := func() {
			_, err := tableTxn.GetOrCompute(txnState, key, computer)
			require.NoError(t, err)
		}
		if meteringEnabled {
			run()
		} else {
			txnState.RunWithMeteringDisabled(run)
		}
		return txnState.ComputationIntensities()[usage.Kind]
	}

	for _, loadEnabled := range []bool{true, false} {
		for _, readEnabled := range []bool{true, false} {
			name := fmt.Sprintf(
				"load(metering=%t)/read(metering=%t)",
				loadEnabled,
				readEnabled)
			t.Run(name, func(t *testing.T) {
				// Cache miss: the reading transaction computes the value
				// itself (cold cache).
				missTable := NewEmptyTable[string, int](0)
				missTxn, err := missTable.NewTableTransaction(0, 0)
				require.NoError(t, err)
				missCharge := charge(t, missTxn, readEnabled)

				// Cache hit: an earlier transaction computed the value within
				// the "load" metering scope (warm cache).
				hitTable := NewEmptyTable[string, int](0)
				loadTxn, err := hitTable.NewTableTransaction(0, 0)
				require.NoError(t, err)
				_ = charge(t, loadTxn, loadEnabled)
				require.NoError(t, loadTxn.Commit())

				hitTxn, err := hitTable.NewTableTransaction(1, 1)
				require.NoError(t, err)
				hitCharge := charge(t, hitTxn, readEnabled)

				require.Equal(
					t,
					missCharge,
					hitCharge,
					"charged computation must not depend on cache state")

				// A read within a metering-disabled scope must not be charged,
				// whether the value is computed or replayed from the cache.
				if !readEnabled {
					require.Zero(
						t,
						missCharge,
						"a metering-disabled read must not be charged")
				} else {
					require.Equal(
						t,
						usage.Intensity,
						missCharge,
						"a metering-enabled read must be fully charged")
				}
			})
		}
	}
}
