package environment_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/storage"
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
		Dependencies: map[common.Location]struct{}{
			programALoc: {},
		},
	}

	addressB := flow.HexToAddress("0xb")
	cAddressB := common.MustBytesToAddress(addressB.Bytes())
	programBLoc := common.AddressLocation{Address: cAddressB, Name: "B"}
	programB := &derived.Program{
		Program: nil,
		Dependencies: map[common.Location]struct{}{
			programALoc: {},
			programBLoc: {},
		},
	}

	addressD := flow.HexToAddress("0xd")
	cAddressD := common.MustBytesToAddress(addressD.Bytes())
	programDLoc := common.AddressLocation{Address: cAddressD, Name: "D"}
	programD := &derived.Program{
		Program: nil,
		Dependencies: map[common.Location]struct{}{
			programDLoc: {},
		},
	}

	addressC := flow.HexToAddress("0xc")
	cAddressC := common.MustBytesToAddress(addressC.Bytes())
	programCLoc := common.AddressLocation{Address: cAddressC, Name: "C"}
	programC := &derived.Program{
		Program: nil,
		Dependencies: map[common.Location]struct{}{
			// C indirectly depends on A trough B
			programALoc: {},
			programBLoc: {},
			programCLoc: {},
			programDLoc: {},
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
			ContractUpdateKeys: []environment.ContractUpdateKey{
				{
					addressA,
					"A",
				},
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
			ContractUpdateKeys: []environment.ContractUpdateKey{
				{
					addressD,
					"D",
				},
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
			ContractUpdateKeys: []environment.ContractUpdateKey{
				{
					addressB,
					"B",
				},
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
					Address: addressA,
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
					Address: addressC,
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
					Address: addressD,
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

	baseView := delta.NewDeltaView(nil)
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

	view := baseView.NewChild()
	nestedTxn := state.NewTransactionState(view, state.DefaultParameters())

	derivedBlockData := derived.NewEmptyDerivedBlockData()
	derivedTxnData, err := derivedBlockData.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	txnState := storage.SerialTransaction{
		NestedTransaction:           nestedTxn,
		DerivedTransactionCommitter: derivedTxnData,
	}
	computer := fvm.NewMeterParamOverridesComputer(ctx, txnState)

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

	checkForUpdates := func(id flow.RegisterID, expected bool) {
		snapshot := &state.ExecutionSnapshot{
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				id: flow.RegisterValue("blah"),
			},
		}

		invalidator := environment.NewDerivedDataInvalidator(
			nil,
			ctx.Chain.ServiceAddress(),
			snapshot)
		require.Equal(t, expected, invalidator.MeterParamOverridesUpdated)
	}

	for _, registerId := range view.Finalize().AllRegisterIDs() {
		checkForUpdates(registerId, true)
		checkForUpdates(
			flow.NewRegisterID("other owner", registerId.Key),
			false)
	}

	owner := string(ctx.Chain.ServiceAddress().Bytes())
	stabIndexKey := flow.NewRegisterID(owner, "$12345678")
	require.True(t, stabIndexKey.IsSlabIndex())

	checkForUpdates(stabIndexKey, true)
	checkForUpdates(flow.NewRegisterID(owner, "other keys"), false)
	checkForUpdates(flow.NewRegisterID("other owner", stabIndexKey.Key), false)
	checkForUpdates(flow.NewRegisterID("other owner", "other key"), false)
}
