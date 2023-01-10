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

	// create the following dependency graph
	// ```mermaid
	// graph TD
	// 	C-->D
	// 	C-->B
	// 	B-->A
	// ```

	addressA := flow.HexToAddress("0xa")
	cAddressA := common.MustBytesToAddress(addressA.Bytes())
	programALoc := common.AddressLocation{Address: cAddressA, Name: "A"}
	programA := &derived.Program{
		Program: nil,
		Dependencies: map[common.Address]struct{}{
			cAddressA: {},
		},
	}

	addressB := flow.HexToAddress("0xb")
	cAddressB := common.MustBytesToAddress(addressB.Bytes())
	programBLoc := common.AddressLocation{Address: cAddressB, Name: "B"}
	programB := &derived.Program{
		Program: nil,
		Dependencies: map[common.Address]struct{}{
			cAddressA: {},
			cAddressB: {},
		},
	}

	addressD := flow.HexToAddress("0xd")
	cAddressD := common.MustBytesToAddress(addressD.Bytes())
	programDLoc := common.AddressLocation{Address: cAddressD, Name: "D"}
	programD := &derived.Program{
		Program: nil,
		Dependencies: map[common.Address]struct{}{
			cAddressD: {},
		},
	}

	addressC := flow.HexToAddress("0xc")
	cAddressC := common.MustBytesToAddress(addressC.Bytes())
	programCLoc := common.AddressLocation{Address: cAddressC, Name: "C"}
	programC := &derived.Program{
		Program: nil,
		Dependencies: map[common.Address]struct{}{
			// C indirectly depends on A trough B
			cAddressA: {},
			cAddressB: {},
			cAddressC: {},
			cAddressD: {},
		},
	}

	t.Run("empty invalidator does not invalidate entries", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{}.ProgramInvalidator()

		require.False(t, invalidator.ShouldInvalidateEntries())
		require.False(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})
	t.Run("meter parameters invalidator invalidates all entries", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{
			MeterParamOverridesUpdated: true,
		}.ProgramInvalidator()

		require.True(t, invalidator.ShouldInvalidateEntries())
		require.True(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})

	t.Run("address invalidator A invalidates all but D", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{
			FrozenAccounts: []common.Address{
				cAddressA,
			},
		}.ProgramInvalidator()

		require.True(t, invalidator.ShouldInvalidateEntries())
		require.True(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})

	t.Run("address invalidator D invalidates D, C", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{
			FrozenAccounts: []common.Address{
				cAddressD,
			},
		}.ProgramInvalidator()

		require.True(t, invalidator.ShouldInvalidateEntries())
		require.False(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})

	t.Run("address invalidator B invalidates B, C", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{
			FrozenAccounts: []common.Address{
				cAddressB,
			},
		}.ProgramInvalidator()

		require.True(t, invalidator.ShouldInvalidateEntries())
		require.False(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})

	t.Run("contract invalidator A invalidates all but D", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{
			ContractUpdateKeys: []environment.ContractUpdateKey{
				{
					Address: cAddressA,
					Name:    "A",
				},
			},
		}.ProgramInvalidator()

		require.True(t, invalidator.ShouldInvalidateEntries())
		require.True(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})

	t.Run("contract invalidator C invalidates C", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{
			ContractUpdateKeys: []environment.ContractUpdateKey{
				{
					Address: cAddressC,
					Name:    "C",
				},
			},
		}.ProgramInvalidator()

		require.True(t, invalidator.ShouldInvalidateEntries())
		require.False(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})

	t.Run("contract invalidator D invalidates C, D", func(t *testing.T) {
		invalidator := environment.DerivedDataInvalidator{
			ContractUpdateKeys: []environment.ContractUpdateKey{
				{
					Address: cAddressD,
					Name:    "D",
				},
			},
		}.ProgramInvalidator()

		require.True(t, invalidator.ShouldInvalidateEntries())
		require.False(t, invalidator.ShouldInvalidateEntry(programALoc, programA, nil))
		require.False(t, invalidator.ShouldInvalidateEntry(programBLoc, programB, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programCLoc, programC, nil))
		require.True(t, invalidator.ShouldInvalidateEntry(programDLoc, programD, nil))
	})
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
