package environment_test

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

var (
	addressA = flow.HexToAddress("0a")
	addressB = flow.HexToAddress("0b")
	addressC = flow.HexToAddress("0c")

	contractALocation = common.AddressLocation{
		Address: common.MustBytesToAddress(addressA.Bytes()),
		Name:    "A",
	}
	contractA2Location = common.AddressLocation{
		Address: common.MustBytesToAddress(addressA.Bytes()),
		Name:    "A2",
	}

	contractBLocation = common.AddressLocation{
		Address: common.MustBytesToAddress(addressB.Bytes()),
		Name:    "B",
	}

	contractCLocation = common.AddressLocation{
		Address: common.MustBytesToAddress(addressC.Bytes()),
		Name:    "C",
	}

	contractA0Code = `
		access(all) contract A {
			access(all) struct interface Foo{}
			
			access(all) fun hello(): String {
        		return "bad version"
    		}
		}
	`

	contractACode = `
		access(all) contract A {
			access(all) struct interface Foo{}

			access(all) fun hello(): String {
        		return "hello from A"
    		}
		}
	`

	contractA2Code = `
		access(all) contract A2 {
			access(all) struct interface Foo{}

			access(all) fun hello(): String {
        		return "hello from A2"
    		}
		}
	`

	contractABreakingCode = `
		access(all) contract A {
			access(all) struct interface Foo{
				access(all) fun hello()
			}
	
			access(all) fun hello(): String {
	  		return "hello from A with breaking change"
			}
		}
	`

	contractBCode = `
		import 0xa

		access(all) contract B {
			access(all) struct Bar : A.Foo {}

			access(all) fun hello(): String {
       			return "hello from B but also ".concat(A.hello())
    		}
		}
	`

	contractCCode = `
		import B from 0xb
		import A from 0xa

		access(all) contract C {
            access(all) struct Bar : A.Foo {}

			access(all) fun hello(): String {
	   			return "hello from C, ".concat(B.hello())
			}
		}
	`
)

func setupProgramsTest(t *testing.T) snapshot.SnapshotTree {
	blockDatabase := storage.NewBlockDatabase(nil, 0, nil)
	txnState, err := blockDatabase.NewTransaction(0, state.DefaultParameters())
	require.NoError(t, err)

	accounts := environment.NewAccounts(txnState)

	err = accounts.Create(nil, addressA)
	require.NoError(t, err)

	err = accounts.Create(nil, addressB)
	require.NoError(t, err)

	err = accounts.Create(nil, addressC)
	require.NoError(t, err)

	executionSnapshot, err := txnState.FinalizeMainTransaction()
	require.NoError(t, err)

	return snapshot.NewSnapshotTree(nil).Append(executionSnapshot)
}

func getTestContract(
	snapshot snapshot.StorageSnapshot,
	location common.AddressLocation,
) (
	[]byte,
	error,
) {
	env := environment.NewScriptEnvironmentFromStorageSnapshot(
		environment.DefaultEnvironmentParams(),
		snapshot)
	return env.GetAccountContractCode(location)
}

func Test_Programs(t *testing.T) {
	vm := fvm.NewVirtualMachine()
	derivedBlockData := derived.NewEmptyDerivedBlockData(0)

	mainSnapshot := setupProgramsTest(t)

	context := fvm.NewContext(
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithCadenceLogging(true),
		fvm.WithDerivedBlockData(derivedBlockData))

	var contractASnapshot *snapshot.ExecutionSnapshot
	var contractBSnapshot *snapshot.ExecutionSnapshot
	var txASnapshot *snapshot.ExecutionSnapshot

	t.Run("contracts can be updated", func(t *testing.T) {
		retrievedContractA, err := getTestContract(
			mainSnapshot,
			contractALocation)
		require.NoError(t, err)
		require.Empty(t, retrievedContractA)

		// deploy contract A0
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("A", contractA0Code, addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		mainSnapshot = mainSnapshot.Append(executionSnapshot)

		retrievedContractA, err = getTestContract(
			mainSnapshot,
			contractALocation)
		require.NoError(t, err)

		require.Equal(t, contractA0Code, string(retrievedContractA))

		// deploy contract A
		executionSnapshot, output, err = vm.Run(
			context,
			fvm.Transaction(
				updateContractTx("A", contractACode, addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		mainSnapshot = mainSnapshot.Append(executionSnapshot)

		retrievedContractA, err = getTestContract(
			mainSnapshot,
			contractALocation)
		require.NoError(t, err)

		require.Equal(t, contractACode, string(retrievedContractA))

	})
	t.Run("register touches are captured for simple contract A", func(t *testing.T) {
		t.Log("---------- Real transaction here ------------")

		// run a TX using contract A

		loadedCode := false
		execASnapshot := snapshot.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				expectedId := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				if id == expectedId {
					loadedCode = true
				}

				return mainSnapshot.Get(id)
			})

		executionSnapshotA, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("A", addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			execASnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		mainSnapshot = mainSnapshot.Append(executionSnapshotA)

		// make sure tx was really run
		require.Contains(t, output.Logs, "\"hello from A\"")

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
		txASnapshot = executionSnapshotA

		// execute transaction again, this time make sure it doesn't load code
		execA2Snapshot := snapshot.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				notId := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				// this time we fail if a read of code occurs
				require.NotEqual(t, id, notId)

				return mainSnapshot.Get(id)
			})

		executionSnapshotA2, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("A", addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			execA2Snapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		mainSnapshot = mainSnapshot.Append(executionSnapshotA2)

		require.Contains(t, output.Logs, "\"hello from A\"")

		// same transaction should produce the exact same execution snapshots
		// but only because we don't do any conditional update in a tx
		compareExecutionSnapshots(t, executionSnapshotA, executionSnapshotA2)
	})

	t.Run("deploying another contract invalidates dependant programs", func(t *testing.T) {
		// deploy contract B
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("B", contractBCode, addressB),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		mainSnapshot = mainSnapshot.Append(executionSnapshot)

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

	t.Run("contract B imports contract A", func(t *testing.T) {

		// programs should have no entries for A and B, as per previous test

		// run a TX using contract B

		executionSnapshotB, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("B", addressB),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)

		mainSnapshot = mainSnapshot.Append(executionSnapshotB)

		require.Contains(t, output.Logs, "\"hello from B but also hello from A\"")

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

		// rerun transaction

		// execute transaction again, this time make sure it doesn't load code
		execB2Snapshot := snapshot.NewReadFuncStorageSnapshot(
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

				return mainSnapshot.Get(id)
			})

		executionSnapshotB2, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("B", addressB),
				derivedBlockData.NextTxIndexForTestingOnly()),
			execB2Snapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Contains(t, output.Logs, "\"hello from B but also hello from A\"")

		mainSnapshot = mainSnapshot.Append(executionSnapshotB2)

		compareExecutionSnapshots(t, executionSnapshotB, executionSnapshotB2)
	})

	t.Run("deploying new contract A2 invalidates B because of * imports", func(t *testing.T) {
		// deploy contract A2
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("A2", contractA2Code, addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		mainSnapshot = mainSnapshot.Append(executionSnapshot)

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

		executionSnapshotB, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("B", addressB),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Contains(t, output.Logs, "\"hello from B but also hello from A\"")

		mainSnapshot = mainSnapshot.Append(executionSnapshotB)

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

		// rerun transaction

		// execute transaction again, this time make sure it doesn't load code
		execB2Snapshot := snapshot.NewReadFuncStorageSnapshot(
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

				return mainSnapshot.Get(id)
			})

		executionSnapshotB2, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("B", addressB),
				derivedBlockData.NextTxIndexForTestingOnly()),
			execB2Snapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Contains(t, output.Logs, "\"hello from B but also hello from A\"")

		mainSnapshot = mainSnapshot.Append(executionSnapshotB2)

		compareExecutionSnapshots(t, executionSnapshotB, executionSnapshotB2)
	})

	t.Run("contract A runs from cache after program B has been loaded", func(t *testing.T) {

		// at this point programs cache should contain data for contract A
		// only because contract B has been called

		execASnapshot := snapshot.NewReadFuncStorageSnapshot(
			func(id flow.RegisterID) (flow.RegisterValue, error) {
				notId := flow.ContractRegisterID(
					flow.BytesToAddress([]byte(id.Owner)),
					"A")
				require.NotEqual(t, id, notId)
				return mainSnapshot.Get(id)
			})

		// run a TX using contract A
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("A", addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			execASnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Contains(t, output.Logs, "\"hello from A\"")

		mainSnapshot = mainSnapshot.Append(executionSnapshot)

		compareExecutionSnapshots(t, txASnapshot, executionSnapshot)
	})

	t.Run("deploying contract C invalidates C", func(t *testing.T) {
		require.NotNil(t, contractBSnapshot)

		// deploy contract C
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("C", contractCCode, addressC),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		mainSnapshot = mainSnapshot.Append(executionSnapshot)

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
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				callTx("C", addressC),
				derivedBlockData.NextTxIndexForTestingOnly()),
			mainSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Contains(t, output.Logs, "\"hello from C, hello from B but also hello from A\"")

		mainSnapshot = mainSnapshot.Append(executionSnapshot)

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
	snapshotTree := setupProgramsTest(t)

	vm := fvm.NewVirtualMachine()
	derivedBlockData := derived.NewEmptyDerivedBlockData(0)

	metrics := &metricsReporter{}
	context := fvm.NewContext(
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithCadenceLogging(true),
		fvm.WithDerivedBlockData(derivedBlockData),
		fvm.WithMetricsReporter(metrics))

	t.Run("deploy contracts and ensure cache is empty", func(t *testing.T) {
		// deploy contract A
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("A", contractACode, addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// deploy contract B
		executionSnapshot, output, err = vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("B", contractBCode, addressB),
				derivedBlockData.NextTxIndexForTestingOnly()),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// deploy contract C
		executionSnapshot, output, err = vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("C", contractCCode, addressC),
				derivedBlockData.NextTxIndexForTestingOnly()),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// deploy contract A2 last to clear any cache so far
		executionSnapshot, output, err = vm.Run(
			context,
			fvm.Transaction(
				contractDeployTx("A2", contractA2Code, addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

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

	callC := func(snapshotTree snapshot.SnapshotTree) snapshot.SnapshotTree {
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

		executionSnapshot, output, err := vm.Run(
			context,
			procCallC,
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Equal(
			t,
			uint(
				1+ // import A
					3+ // import B (import A, import A2)
					4, // import C (import B (3), import A (already imported in this scope))
			),
			output.ComputationIntensities[environment.ComputationKindGetCode])

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

		return snapshotTree.Append(executionSnapshot)
	}

	t.Run("Call C", func(t *testing.T) {
		metrics.Reset()
		snapshotTree = callC(snapshotTree)

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
		snapshotTree = callC(snapshotTree)

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

	t.Run("update A to breaking change and ensure cache state", func(t *testing.T) {
		// deploy contract A
		executionSnapshot, output, err := vm.Run(
			context,
			fvm.Transaction(
				updateContractTx("A", contractABreakingCode, addressA),
				derivedBlockData.NextTxIndexForTestingOnly()),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)

		require.Nil(t, entryA)
		require.Nil(t, entryB)
		require.Nil(t, entryC)

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 1, cached)
	})

	callCAfterItsBroken := func(snapshotTree snapshot.SnapshotTree) snapshot.SnapshotTree {
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

		executionSnapshot, output, err := vm.Run(
			context,
			procCallC,
			snapshotTree)
		require.NoError(t, err)
		require.Error(t, output.Err)

		entryA := derivedBlockData.GetProgramForTestingOnly(contractALocation)
		entryA2 := derivedBlockData.GetProgramForTestingOnly(contractA2Location)
		entryB := derivedBlockData.GetProgramForTestingOnly(contractBLocation)
		entryC := derivedBlockData.GetProgramForTestingOnly(contractCLocation)

		require.NotNil(t, entryA)
		require.NotNil(t, entryA2) // loaded due to "*" import in B
		require.Nil(t, entryB)     // failed to load
		require.Nil(t, entryC)     // failed to load

		cached := derivedBlockData.CachedPrograms()
		require.Equal(t, 2, cached)

		return snapshotTree.Append(executionSnapshot)
	}

	t.Run("Call C when broken", func(t *testing.T) {
		metrics.Reset()
		snapshotTree = callCAfterItsBroken(snapshotTree)

		// miss A, hit A, hit A2, hit A, hit A2, hit A
		require.Equal(t, 5, metrics.CacheHits)
		require.Equal(t, 1, metrics.CacheMisses)
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
              prepare(signer: auth(AddContract) &Account) {
                signer.contracts.add(name: "%s", code: "%s".decodeHex())
              }
            }`, name, encoded)),
	).AddAuthorizer(address)
}

func updateContractTx(name, code string, address flow.Address) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(code))

	return flow.NewTransactionBody().SetScript([]byte(
		fmt.Sprintf(`transaction {
             prepare(signer: auth(UpdateContract) &Account) {
               signer.contracts.update(name: "%s", code: "%s".decodeHex())
             }
           }`, name, encoded)),
	).AddAuthorizer(address)
}

func compareExecutionSnapshots(t *testing.T, a, b *snapshot.ExecutionSnapshot) {
	require.Equal(t, a.WriteSet, b.WriteSet)
	require.Equal(t, a.ReadSet, b.ReadSet)
	require.Equal(t, a.SpockSecret, b.SpockSecret)
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
