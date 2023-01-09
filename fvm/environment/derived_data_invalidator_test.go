package environment_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDerivedDataProgramInvalidator(t *testing.T) {
	invalidator := environment.DerivedDataInvalidator{}.ProgramInvalidator()

	require.False(t, invalidator.ShouldInvalidateEntries())
	require.False(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	invalidator = environment.DerivedDataInvalidator{
		ContractUpdateKeys: []environment.ContractUpdateKey{
			{}, // For now, the entry's value does not matter.
		},
		FrozenAccounts:             nil,
		MeterParamOverridesUpdated: false,
	}.ProgramInvalidator()

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	invalidator = environment.DerivedDataInvalidator{
		ContractUpdateKeys: nil,
		FrozenAccounts: []common.Address{
			{}, // For now, the entry's value does not matter
		},
		MeterParamOverridesUpdated: false,
	}.ProgramInvalidator()

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	invalidator = environment.DerivedDataInvalidator{
		ContractUpdateKeys:         nil,
		FrozenAccounts:             nil,
		MeterParamOverridesUpdated: true,
	}.ProgramInvalidator()

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))
}

func TestMeterParamOverridesInvalidator(t *testing.T) {
	invalidator := environment.DerivedDataInvalidator{}.
		MeterParamOverridesInvalidator()

	require.False(t, invalidator.ShouldInvalidateEntries())
	require.False(t, invalidator.ShouldInvalidateEntry(
		struct{}{},
		derived.MeterParamOverrides{},
		nil))

	invalidator = environment.DerivedDataInvalidator{
		ContractUpdateKeys:         nil,
		FrozenAccounts:             nil,
		MeterParamOverridesUpdated: true,
	}.MeterParamOverridesInvalidator()

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		struct{}{},
		derived.MeterParamOverrides{},
		nil))
}

func TestMeterParamOverridesUpdated(t *testing.T) {
	memoryLimit := uint64(666)

	compKind := common.ComputationKind(12345)
	compWeight := uint64(10)
	computationWeights := meter.ExecutionEffortWeights{
		compKind: compWeight,
	}

	memKind := common.MemoryKind(23456)
	memWeight := uint64(20000)
	memoryWeights := meter.ExecutionMemoryWeights{
		memKind: memWeight,
	}

	baseView := utils.NewSimpleView()
	ctx := fvm.NewContext(fvm.WithChain(flow.Testnet.Chain()))

	vm := fvm.NewVirtualMachine()
	err := vm.Run(
		ctx,
		fvm.Bootstrap(
			unittest.ServiceAccountPublicKey,
			fvm.WithExecutionMemoryLimit(memoryLimit),
			fvm.WithExecutionEffortWeights(computationWeights),
			fvm.WithExecutionMemoryWeights(memoryWeights)),
		baseView)
	require.NoError(t, err)

	view := baseView.NewChild().(*utils.SimpleView)
	txnState := state.NewTransactionState(view, state.DefaultParameters())

	derivedBlockData := derived.NewEmptyDerivedBlockData()
	derivedTxnData, err := derivedBlockData.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	computer := fvm.NewMeterParamOverridesComputer(ctx, derivedTxnData)

	overrides, err := computer.Compute(txnState, struct{}{})
	require.NoError(t, err)

	// Sanity check.  Note that bootstrap creates additional computation /
	// memory weight entries.  We'll only check the entries we added.
	require.NotNil(t, overrides.MemoryLimit)
	require.Equal(t, memoryLimit, *overrides.MemoryLimit)
	require.Equal(t, compWeight, overrides.ComputationWeights[compKind])
	require.Equal(t, memWeight, overrides.MemoryWeights[memKind])

	//
	// Actual test
	//

	ctx.TxBody = &flow.TransactionBody{}

	checkForUpdates := func(owner string, key string, expected bool) {
		view := utils.NewSimpleView()
		txnState := state.NewTransactionState(view, state.DefaultParameters())

		err := txnState.Set(owner, key, flow.RegisterValue("blah"), false)
		require.NoError(t, err)

		env := environment.NewTransactionEnvironment(
			tracing.NewTracerSpan(),
			ctx.EnvironmentParams,
			txnState,
			nil)

		invalidator := environment.NewDerivedDataInvalidator(nil, env)
		require.Equal(t, expected, invalidator.MeterParamOverridesUpdated)
	}

	for registerId := range view.Ledger.RegisterTouches {
		checkForUpdates(registerId.Owner, registerId.Key, true)
		checkForUpdates("other owner", registerId.Key, false)
	}

	stabIndex := "$12345678"
	require.True(t, state.IsSlabIndex(stabIndex))

	checkForUpdates(
		string(ctx.Chain.ServiceAddress().Bytes()),
		stabIndex,
		true)

	checkForUpdates(
		string(ctx.Chain.ServiceAddress().Bytes()),
		"other keys",
		false)

	checkForUpdates("other owner", stabIndex, false)

	checkForUpdates("other owner", "other key", false)
}
