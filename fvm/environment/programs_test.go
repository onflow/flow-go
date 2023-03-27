package environment_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/model/flow"
)

func Test_Programs(t *testing.T) {

	addressA := flow.HexToAddress("0a")
	addressB := flow.HexToAddress("0b")
	addressC := flow.HexToAddress("0c")

	contractALocation := common.AddressLocation{
		Address: common.MustBytesToAddress(addressA.Bytes()),
		Name:    "A",
	}
	contractA2Location := common.AddressLocation{
		Address: common.MustBytesToAddress(addressA.Bytes()),
		Name:    "A2",
	}

	contractBLocation := common.AddressLocation{
		Address: common.MustBytesToAddress(addressB.Bytes()),
		Name:    "B",
	}

	contractCLocation := common.AddressLocation{
		Address: common.MustBytesToAddress(addressC.Bytes()),
		Name:    "C",
	}

	contractA0Code := `
		pub contract A {
			pub fun hello(): String {
        		return "bad version"
    		}
		}
	`

	contractACode := `
		pub contract A {
			pub fun hello(): String {
        		return "hello from A"
    		}
		}
	`

	contractA2Code := `
		pub contract A2 {
			pub fun hello(): String {
        		return "hello from A2"
    		}
		}
	`

	contractBCode := `
		import 0xa

		pub contract B {
			pub fun hello(): String {
       			return "hello from B but also ".concat(A.hello())
    		}
		}
	`

	contractCCode := `
		import B from 0xb

		pub contract C {
			pub fun hello(): String {
	   			return "hello from C, ".concat(B.hello())
			}
		}
	`

	mainView := delta.NewDeltaView(nil)

	vm := fvm.NewVirtualMachine()
	derivedBlockData := derived.NewEmptyDerivedBlockData()

	accounts := environment.NewAccounts(
		storage.SerialTransaction{
			NestedTransaction: state.NewTransactionState(
				mainView,
				state.DefaultParameters()),
		})

	err := accounts.Create(nil, addressA)
	require.NoError(t, err)

	err = accounts.Create(nil, addressB)
	require.NoError(t, err)

	err = accounts.Create(nil, addressC)
	require.NoError(t, err)

	// err = stm.
	require.NoError(t, err)

	fmt.Printf("Account created\n")

	context := fvm.NewContext(
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithCadenceLogging(true),
		fvm.WithDerivedBlockData(derivedBlockData))

	var contractASnapshot *state.ExecutionSnapshot
	var contractBSnapshot *state.ExecutionSnapshot
	var txAView *delta.View = nil

	t.Run("contracts can be updated", func(t *testing.T) {
		retrievedContractA, err := accounts.GetContract("A", addressA)
		require.NoError(t, err)
		require.Empty(t, retrievedContractA)

		// deploy contract A0
		procContractA0 := fvm.Transaction(
			contractDeployTx("A", contractA0Code, addressA),
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, procContractA0, mainView)
		require.NoError(t, err)

		retrievedContractA, err = accounts.GetContract("A", addressA)
		require.NoError(t, err)

		require.Equal(t, contractA0Code, string(retrievedContractA))

		// deploy contract A
		procContractA := fvm.Transaction(
			updateContractTx("A", contractACode, addressA),
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, procContractA, mainView)
		require.NoError(t, err)
		require.NoError(t, procContractA.Err)

		retrievedContractA, err = accounts.GetContract("A", addressA)
		require.NoError(t, err)

		require.Equal(t, contractACode, string(retrievedContractA))

	})

	t.Run("register touches are captured for simple contract A", func(t *testing.T) {

		// deploy contract A
		procContractA := fvm.Transaction(
			contractDeployTx("A", contractACode, addressA),
			derivedBlockData.NextTxIndexForTestingOnly())
		err := vm.Run(context, procContractA, mainView)
		require.NoError(t, err)

		fmt.Println("---------- Real transaction here ------------")

		// run a TX using contract A
		procCallA := fvm.Transaction(
			callTx("A", addressA),
			derivedBlockData.NextTxIndexForTestingOnly())

		loadedCode := false
		viewExecA := delta.NewDeltaView(state.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				expectedId := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				if id == expectedId {
					loadedCode = true
				}

				return mainView.Peek(id)
			}))

		err = vm.Run(context, procCallA, viewExecA)
		require.NoError(t, err)

		// make sure tx was really run
		require.Contains(t, procCallA.Logs, "\"hello from A\"")

		// Make sure the code has been loaded from storage
		require.True(t, loadedCode)

		entry := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		require.NotNil(t, entry)
		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 1, cached)

		// assert dependencies are correct
		require.Equal(t, 1, entry.Value.Dependencies.Count())
		require.True(t, entry.Value.Dependencies.ContainsLocation(contractALocation))

		// assert some reads were recorded (at least loading of code)
		require.NotEmpty(t, entry.ExecutionSnapshot.ReadSet)

		contractASnapshot = entry.ExecutionSnapshot
		txAView = viewExecA

		// merge it back
		err = mainView.Merge(viewExecA.Finalize())
		require.NoError(t, err)

		// execute transaction again, this time make sure it doesn't load code
		viewExecA2 := delta.NewDeltaView(state.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				notId := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				// this time we fail if a read of code occurs
				require.NotEqual(t, id, notId)

				return mainView.Peek(id)
			}))

		procCallA = fvm.Transaction(
			callTx("A", addressA),
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, procCallA, viewExecA2)
		require.NoError(t, err)

		require.Contains(t, procCallA.Logs, "\"hello from A\"")

		// same transaction should produce the exact same views
		// but only because we don't do any conditional update in a tx
		compareViews(t, viewExecA, viewExecA2)

		// merge it back
		err = mainView.Merge(viewExecA2.Finalize())
		require.NoError(t, err)
	})

	t.Run("deploying another contract invalidates dependant programs", func(t *testing.T) {
		// deploy contract B
		procContractB := fvm.Transaction(
			contractDeployTx("B", contractBCode, addressB),
			derivedBlockData.NextTxIndexForTestingOnly())
		err := vm.Run(context, procContractB, mainView)
		require.NoError(t, err)

		// b and c are invalid
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)
		// a is still valid
		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)

		require.Nil(t, entryB)
		require.Nil(t, entryC)
		require.NotNil(t, entryA)

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 1, cached)
	})

	var viewExecB *delta.View

	t.Run("contract B imports contract A", func(t *testing.T) {

		// programs should have no entries for A and B, as per previous test

		// run a TX using contract B
		procCallB := fvm.Transaction(
			callTx("B", addressB),
			derivedBlockData.NextTxIndexForTestingOnly())

		viewExecB = delta.NewDeltaView(
			state.NewPeekerStorageSnapshot(mainView))

		err = vm.Run(context, procCallB, viewExecB)
		require.NoError(t, err)

		require.Contains(t, procCallB.Logs, "\"hello from B but also hello from A\"")

		entry := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		require.NotNil(t, entry)

		// state should be essentially the same as one which we got in tx with contract A
		require.Equal(t, contractASnapshot, entry.ExecutionSnapshot)

		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		require.NotNil(t, entryB)

		// assert dependencies are correct
		require.Equal(t, 2, entryB.Value.Dependencies.Count())
		require.NotNil(t, entryB.Value.Dependencies.ContainsLocation(contractALocation))
		require.NotNil(t, entryB.Value.Dependencies.ContainsLocation(contractBLocation))

		// program B should contain all the registers used by program A, as it depends on it
		contractBSnapshot = entryB.ExecutionSnapshot

		require.Empty(t, contractASnapshot.WriteSet)

		for id := range contractASnapshot.ReadSet {
			_, ok := contractBSnapshot.ReadSet[id]
			require.True(t, ok)
		}

		// merge it back
		err = mainView.Merge(viewExecB.Finalize())
		require.NoError(t, err)

		// rerun transaction

		// execute transaction again, this time make sure it doesn't load code
		viewExecB2 := delta.NewDeltaView(state.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				idA := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				idB := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"B")
				// this time we fail if a read of code occurs
				require.NotEqual(t, id.Key, idA.Key)
				require.NotEqual(t, id.Key, idB.Key)

				return mainView.Peek(id)
			}))

		procCallB = fvm.Transaction(
			callTx("B", addressB),
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, procCallB, viewExecB2)
		require.NoError(t, err)

		require.Contains(t, procCallB.Logs, "\"hello from B but also hello from A\"")

		compareViews(t, viewExecB, viewExecB2)

		// merge it back
		err = mainView.Merge(viewExecB2.Finalize())
		require.NoError(t, err)
	})

	t.Run("deploying new contract A2 invalidates B because of * imports", func(t *testing.T) {
		// deploy contract B
		procContractA2 := fvm.Transaction(
			contractDeployTx("A2", contractA2Code, addressA),
			derivedBlockData.NextTxIndexForTestingOnly())
		err := vm.Run(context, procContractA2, mainView)
		require.NoError(t, err)

		// a, b and c are invalid
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)
		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)

		require.Nil(t, entryB) // B could have star imports to 0xa, so it's invalidated
		require.Nil(t, entryC) // still invalid
		require.Nil(t, entryA) // A could have star imports to 0xa, so it's invalidated

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 0, cached)
	})

	t.Run("contract B imports contract A and A2 because of * import", func(t *testing.T) {

		// programs should have no entries for A and B, as per previous test

		// run a TX using contract B
		procCallB := fvm.Transaction(
			callTx("B", addressB),
			derivedBlockData.NextTxIndexForTestingOnly())

		viewExecB = delta.NewDeltaView(
			state.NewPeekerStorageSnapshot(mainView))

		err = vm.Run(context, procCallB, viewExecB)
		require.NoError(t, err)

		require.Contains(t, procCallB.Logs, "\"hello from B but also hello from A\"")

		entry := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		require.NotNil(t, entry)

		// state should be essentially the same as one which we got in tx with contract A
		require.Equal(t, contractASnapshot, entry.ExecutionSnapshot)

		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		require.NotNil(t, entryB)

		// assert dependencies are correct
		require.Equal(t, 3, entryB.Value.Dependencies.Count())
		require.NotNil(t, entryB.Value.Dependencies.ContainsLocation(contractALocation))
		require.NotNil(t, entryB.Value.Dependencies.ContainsLocation(contractBLocation))
		require.NotNil(t, entryB.Value.Dependencies.ContainsLocation(contractA2Location))

		// program B should contain all the registers used by program A, as it depends on it
		contractBSnapshot = entryB.ExecutionSnapshot

		require.Empty(t, contractASnapshot.WriteSet)

		for id := range contractASnapshot.ReadSet {
			_, ok := contractBSnapshot.ReadSet[id]
			require.True(t, ok)
		}

		// merge it back
		err = mainView.Merge(viewExecB.Finalize())
		require.NoError(t, err)

		// rerun transaction

		// execute transaction again, this time make sure it doesn't load code
		viewExecB2 := delta.NewDeltaView(state.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				idA := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				idA2 := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A2")
				idB := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"B")
				// this time we fail if a read of code occurs
				require.NotEqual(t, id.Key, idA.Key)
				require.NotEqual(t, id.Key, idA2.Key)
				require.NotEqual(t, id.Key, idB.Key)

				return mainView.Peek(id)
			}))

		procCallB = fvm.Transaction(
			callTx("B", addressB),
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, procCallB, viewExecB2)
		require.NoError(t, err)

		require.Contains(t, procCallB.Logs, "\"hello from B but also hello from A\"")

		compareViews(t, viewExecB, viewExecB2)

		// merge it back
		err = mainView.Merge(viewExecB2.Finalize())
		require.NoError(t, err)
	})

	t.Run("contract A runs from cache after program B has been loaded", func(t *testing.T) {

		// at this point programs cache should contain data for contract A
		// only because contract B has been called

		viewExecA := delta.NewDeltaView(state.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				notId := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				require.NotEqual(t, id, notId)
				return mainView.Peek(id)
			}))

		// run a TX using contract A
		procCallA := fvm.Transaction(
			callTx("A", addressA),
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, procCallA, viewExecA)
		require.NoError(t, err)

		require.Contains(t, procCallA.Logs, "\"hello from A\"")

		compareViews(t, txAView, viewExecA)

		// merge it back
		err = mainView.Merge(viewExecA.Finalize())
		require.NoError(t, err)
	})

	t.Run("deploying contract C invalidates C", func(t *testing.T) {
		require.NotNil(t, contractBSnapshot)

		// deploy contract C
		procContractC := fvm.Transaction(
			contractDeployTx("C", contractCCode, addressC),
			derivedBlockData.NextTxIndexForTestingOnly())
		err := vm.Run(context, procContractC, mainView)
		require.NoError(t, err)

		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		entryA2 := derivedBlockData.GetProgramForTestingOnly(contractA2Location)
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)

		require.NotNil(t, entryA)
		require.NotNil(t, entryA2)
		require.NotNil(t, entryB)
		require.Nil(t, entryC)

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 3, cached)
	})

	t.Run("importing C should chain-import B and A", func(t *testing.T) {
		procCallC := fvm.Transaction(
			callTx("C", addressC),
			derivedBlockData.NextTxIndexForTestingOnly())

		viewExecC := delta.NewDeltaView(
			state.NewPeekerStorageSnapshot(mainView))

		err = vm.Run(context, procCallC, viewExecC)
		require.NoError(t, err)

		require.Contains(t, procCallC.Logs, "\"hello from C, hello from B but also hello from A\"")

		// program A is the same
		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		require.NotNil(t, entryA)

		require.Equal(t, contractASnapshot, entryA.ExecutionSnapshot)

		// program B is the same
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		require.NotNil(t, entryB)

		require.Equal(t, contractBSnapshot, entryB.ExecutionSnapshot)

		// program C assertions
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)
		require.NotNil(t, entryC)

		// assert dependencies are correct
		require.Equal(t, 4, entryC.Value.Dependencies.Count())
		require.NotNil(t, entryC.Value.Dependencies.ContainsLocation(contractALocation))
		require.NotNil(t, entryC.Value.Dependencies.ContainsLocation(contractBLocation))
		require.NotNil(t, entryC.Value.Dependencies.ContainsLocation(contractCLocation))

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 4, cached)
	})
}

func Test_ProgramsDoubleCounting(t *testing.T) {

	addressA := flow.HexToAddress("0a")
	addressB := flow.HexToAddress("0b")
	addressC := flow.HexToAddress("0c")

	contractALocation := common.AddressLocation{
		Address: common.MustBytesToAddress(addressA.Bytes()),
		Name:    "A",
	}
	contractA2Location := common.AddressLocation{
		Address: common.MustBytesToAddress(addressA.Bytes()),
		Name:    "A2",
	}

	contractBLocation := common.AddressLocation{
		Address: common.MustBytesToAddress(addressB.Bytes()),
		Name:    "B",
	}

	contractCLocation := common.AddressLocation{
		Address: common.MustBytesToAddress(addressC.Bytes()),
		Name:    "C",
	}

	contractACode := `
		pub contract A {
			pub fun hello(): String {
        		return "hello from A"
    		}
		}
	`

	contractA2Code := `
		pub contract A2 {
			pub fun hello(): String {
        		return "hello from A2"
    		}
		}
	`

	contractBCode := `
		import 0xa

		pub contract B {
			pub fun hello(): String {
       			return "hello from B but also ".concat(A.hello())
    		}
		}
	`

	contractCCode := `
		import B from 0xb
		import A from 0xa

		pub contract C {
			pub fun hello(): String {
	   			return "hello from C, ".concat(B.hello())
			}
		}
	`

	mainView := delta.NewDeltaView(nil)

	vm := fvm.NewVirtualMachine()
	derivedBlockData := derived.NewEmptyDerivedBlockData()

	accounts := environment.NewAccounts(
		storage.SerialTransaction{
			NestedTransaction: state.NewTransactionState(
				mainView,
				state.DefaultParameters()),
		})

	err := accounts.Create(nil, addressA)
	require.NoError(t, err)

	err = accounts.Create(nil, addressB)
	require.NoError(t, err)

	err = accounts.Create(nil, addressC)
	require.NoError(t, err)

	// err = stm.
	require.NoError(t, err)

	fmt.Printf("Account created\n")

	metrics := &metricsReporter{}
	context := fvm.NewContext(
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithCadenceLogging(true),
		fvm.WithDerivedBlockData(derivedBlockData),
		fvm.WithMetricsReporter(metrics))

	t.Run("deploy contracts and ensure cache is empty", func(t *testing.T) {

		view := delta.NewDeltaView(state.NewPeekerStorageSnapshot(mainView))

		// deploy contract A
		procContractA := fvm.Transaction(
			contractDeployTx("A", contractACode, addressA),
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, procContractA, view)
		require.NoError(t, err)
		require.NoError(t, procContractA.Err)

		// deploy contract B
		procContractB := fvm.Transaction(
			contractDeployTx("B", contractBCode, addressB),
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, procContractB, view)
		require.NoError(t, err)
		require.NoError(t, procContractB.Err)

		// deploy contract C
		procContractC := fvm.Transaction(
			contractDeployTx("C", contractCCode, addressC),
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, procContractC, view)
		require.NoError(t, err)
		require.NoError(t, procContractC.Err)

		// deploy contract A2 last to clear any cache so far
		procContractA2 := fvm.Transaction(
			contractDeployTx("A2", contractA2Code, addressA),
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, procContractA2, view)
		require.NoError(t, err)
		require.NoError(t, procContractA2.Err)

		// merge it back
		err = mainView.Merge(view.Finalize())
		require.NoError(t, err)

		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		entryA2 := derivedBlockData.GetProgramForTestingOnly(contractA2Location)
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)

		require.Nil(t, entryA)
		require.Nil(t, entryA2)
		require.Nil(t, entryB)
		require.Nil(t, entryC)

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 0, cached)
	})

	callC := func() {
		view := delta.NewDeltaView(state.NewPeekerStorageSnapshot(mainView))

		procCallC := fvm.Transaction(
			flow.NewTransactionBody().SetScript(
				[]byte(
					`
					import A from 0xa
					import B from 0xb
					import C from 0xc
					transaction {
						prepare() {
							log(C.hello())
						}
					}`,
				)),
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, procCallC, view)
		require.NoError(t, err)
		require.NoError(t, procCallC.Err)

		require.Equal(t, uint(
			1+ // import A
				3+ // import B (import A, import A2)
				4, // import C (import B (3), import A (already imported in this scope))
		), procCallC.ComputationIntensities[environment.ComputationKindGetCode])

		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		entryA2 := derivedBlockData.GetProgramForTestingOnly(contractA2Location)
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)

		require.NotNil(t, entryA)
		require.NotNil(t, entryA2) // loaded due to "*" import
		require.NotNil(t, entryB)
		require.NotNil(t, entryC)

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 4, cached)

		err = mainView.Merge(view.Finalize())
		require.NoError(t, err)
	}

	t.Run("Call C", func(t *testing.T) {
		metrics.Reset()
		callC()

		// miss A because loading transaction
		// hit A because loading B because loading transaction
		// miss A2 because loading B because loading transaction
		// miss B because loading transaction
		// hit B because loading C because loading transaction
		// hit A because loading C because loading transaction
		// miss C because loading transaction
		//
		// hit C because interpreting transaction
		// hit B because interpreting C because interpreting transaction
		// hit A because interpreting B because interpreting C because interpreting transaction
		// hit A2 because interpreting B because interpreting C because interpreting transaction
		require.Equal(t, 7, metrics.CacheHits)
		require.Equal(t, 4, metrics.CacheMisses)
	})

	t.Run("Call C Again", func(t *testing.T) {
		metrics.Reset()
		callC()

		// hit A because loading transaction
		// hit B because loading transaction
		// hit C because loading transaction
		//
		// hit C because interpreting transaction
		// hit B because interpreting C because interpreting transaction
		// hit A because interpreting B because interpreting C because interpreting transaction
		// hit A2 because interpreting B because interpreting C because interpreting transaction
		require.Equal(t, 7, metrics.CacheHits)
		require.Equal(t, 0, metrics.CacheMisses)
	})

}

func callTx(name string, address flow.Address) *flow.TransactionBody {

	return flow.NewTransactionBody().SetScript(
		[]byte(fmt.Sprintf(`
			import %s from %s
			transaction {
              prepare() {
                log(%s.hello())
              }
            }`, name, address.HexWithPrefix(), name)),
	)
}

func contractDeployTx(name, code string, address flow.Address) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(code))

	return flow.NewTransactionBody().SetScript(
		[]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.contracts.add(name: "%s", code: "%s".decodeHex())
              }
            }`, name, encoded)),
	).AddAuthorizer(address)
}

func updateContractTx(name, code string, address flow.Address) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(code))

	return flow.NewTransactionBody().SetScript([]byte(
		fmt.Sprintf(`transaction {
             prepare(signer: AuthAccount) {
               signer.contracts.update__experimental(name: "%s", code: "%s".decodeHex())
             }
           }`, name, encoded)),
	).AddAuthorizer(address)
}

// compareViews compares views using only data that matters (ie. two different hasher instances
// trips the library comparison, even if actual SPoCKs are the same)
func compareViews(t *testing.T, a, b *delta.View) {
	require.Equal(t, a.Delta(), b.Delta())
	require.Equal(t, a.Interactions(), b.Interactions())
	require.Equal(t, a.SpockSecret(), b.SpockSecret())
}

type metricsReporter struct {
	CacheHits   int
	CacheMisses int
}

func (m *metricsReporter) RuntimeTransactionParsed(duration time.Duration) {}

func (m *metricsReporter) RuntimeTransactionChecked(duration time.Duration) {}

func (m *metricsReporter) RuntimeTransactionInterpreted(duration time.Duration) {}

func (m *metricsReporter) RuntimeSetNumberOfAccounts(count uint64) {}

func (m *metricsReporter) RuntimeTransactionProgramsCacheMiss() {
	m.CacheMisses++
}

func (m *metricsReporter) RuntimeTransactionProgramsCacheHit() {
	m.CacheHits++
}

func (m *metricsReporter) Reset() {
	m.CacheHits = 0
	m.CacheMisses = 0
}

var _ environment.MetricsReporter = (*metricsReporter)(nil)
