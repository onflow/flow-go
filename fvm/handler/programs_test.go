package handler_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	programsStorage "github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func Test_Programs(t *testing.T) {

	addressA := flow.HexToAddress("0a")
	addressB := flow.HexToAddress("0b")
	addressC := flow.HexToAddress("0c")

	contractALocation := common.AddressLocation{
		Address: common.Address(addressA),
		Name:    "A",
	}

	contractBLocation := common.AddressLocation{
		Address: common.Address(addressB),
		Name:    "B",
	}

	contractCLocation := common.AddressLocation{
		Address: common.Address(addressC),
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

	contractBCode := `
		import A from 0xa
	
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

	callTx := func(name string, address flow.Address) *flow.TransactionBody {

		return flow.NewTransactionBody().SetScript([]byte(fmt.Sprintf(`
			import %s from %s
			transaction {
              prepare() {
                log(%s.hello())
              }
            }`, name, address.HexWithPrefix(), name)),
		)
	}

	contractDeployTx := func(name, code string, address flow.Address) *flow.TransactionBody {
		encoded := hex.EncodeToString([]byte(code))

		return flow.NewTransactionBody().SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.contracts.add(name: "%s", code: "%s".decodeHex())
              }
            }`, name, encoded)),
		).AddAuthorizer(address)
	}

	updateContractTx := func(name, code string, address flow.Address) *flow.TransactionBody {
		encoded := hex.EncodeToString([]byte(code))

		return flow.NewTransactionBody().SetScript([]byte(fmt.Sprintf(`transaction {
             prepare(signer: AuthAccount) {
               signer.contracts.update__experimental(name: "%s", code: "%s".decodeHex())
             }
           }`, name, encoded)),
		).AddAuthorizer(address)
	}

	mainView := delta.NewView(func(_, _, _ string) (flow.RegisterValue, error) {
		return nil, nil
	})

	sth := state.NewStateHolder(state.NewState(mainView))

	rt := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(rt)
	programs := programsStorage.NewEmptyPrograms()

	accounts := state.NewAccounts(sth)

	err := accounts.Create(nil, addressA)
	require.NoError(t, err)

	err = accounts.Create(nil, addressB)
	require.NoError(t, err)

	err = accounts.Create(nil, addressC)
	require.NoError(t, err)

	//err = stm.
	require.NoError(t, err)

	fmt.Printf("Account created\n")

	context := fvm.NewContext(zerolog.Nop(), fvm.WithRestrictedDeployment(false), fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())), fvm.WithCadenceLogging(true))

	var contractAView *delta.View = nil
	var contractBView *delta.View = nil
	var txAView *delta.View = nil

	t.Run("contracts can be updated", func(t *testing.T) {
		retrievedContractA, err := accounts.GetContract("A", addressA)
		require.NoError(t, err)
		require.Empty(t, retrievedContractA)

		// deploy contract A0
		procContractA0 := fvm.Transaction(contractDeployTx("A", contractA0Code, addressA), 0)
		err = vm.Run(context, procContractA0, mainView, programs)
		require.NoError(t, err)

		retrievedContractA, err = accounts.GetContract("A", addressA)
		require.NoError(t, err)

		require.Equal(t, contractA0Code, string(retrievedContractA))

		// deploy contract A
		procContractA := fvm.Transaction(updateContractTx("A", contractACode, addressA), 1)
		err = vm.Run(context, procContractA, mainView, programs)
		require.NoError(t, err)
		require.NoError(t, procContractA.Err)

		retrievedContractA, err = accounts.GetContract("A", addressA)
		require.NoError(t, err)

		require.Equal(t, contractACode, string(retrievedContractA))

	})

	t.Run("register touches are captured for simple contract A", func(t *testing.T) {

		// deploy contract A
		procContractA := fvm.Transaction(contractDeployTx("A", contractACode, addressA), 0)
		err := vm.Run(context, procContractA, mainView, programs)
		require.NoError(t, err)

		fmt.Println("---------- Real transaction here ------------")

		// run a TX using contract A
		procCallA := fvm.Transaction(callTx("A", addressA), 1)

		loadedCode := false
		viewExecA := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			if key == state.ContractKey("A") {
				loadedCode = true
			}

			return mainView.Peek(owner, controller, key)
		})

		err = vm.Run(context, procCallA, viewExecA, programs)
		require.NoError(t, err)

		// make sure tx was really run
		require.Contains(t, procCallA.Logs, "\"hello from A\"")

		// Make sure the code has been loaded from storage
		require.True(t, loadedCode)

		_, programState, has := programs.Get(contractALocation)
		require.True(t, has)

		// type assertion for further inspections
		require.IsType(t, programState.View(), &delta.View{})

		// assert some reads were recorded (at least loading of code)
		deltaView := programState.View().(*delta.View)
		require.NotEmpty(t, deltaView.Interactions().Reads)

		contractAView = deltaView
		txAView = viewExecA

		// merge it back
		err = mainView.MergeView(viewExecA)
		require.NoError(t, err)

		// execute transaction again, this time make sure it doesn't load code
		viewExecA2 := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			//this time we fail if a read of code occurs
			require.NotEqual(t, key, state.ContractKey("A"))

			return mainView.Peek(owner, controller, key)
		})

		procCallA = fvm.Transaction(callTx("A", addressA), 2)

		err = vm.Run(context, procCallA, viewExecA2, programs)
		require.NoError(t, err)

		require.Contains(t, procCallA.Logs, "\"hello from A\"")

		// same transaction should produce the exact same views
		// but only because we don't do any conditional update in a tx
		compareViews(t, viewExecA, viewExecA2)

		// merge it back
		err = mainView.MergeView(viewExecA2)
		require.NoError(t, err)
	})

	t.Run("deploying another contract cleans programs storage", func(t *testing.T) {

		// deploy contract B
		procContractB := fvm.Transaction(contractDeployTx("B", contractBCode, addressB), 3)
		err := vm.Run(context, procContractB, mainView, programs)
		require.NoError(t, err)

		_, _, hasA := programs.Get(contractALocation)
		_, _, hasB := programs.Get(contractBLocation)

		require.False(t, hasA)
		require.False(t, hasB)
	})

	var viewExecB *delta.View

	t.Run("contract B imports contract A", func(t *testing.T) {

		// programs should have no entries for A and B, as per previous test

		// run a TX using contract B
		procCallB := fvm.Transaction(callTx("B", addressB), 4)

		viewExecB = delta.NewView(mainView.Peek)

		err = vm.Run(context, procCallB, viewExecB, programs)
		require.NoError(t, err)

		require.Contains(t, procCallB.Logs, "\"hello from B but also hello from A\"")

		_, programAState, has := programs.Get(contractALocation)
		require.True(t, has)

		// state should be essentially the same as one which we got in tx with contract A
		require.IsType(t, programAState.View(), &delta.View{})
		deltaA := programAState.View().(*delta.View)

		compareViews(t, contractAView, deltaA)

		_, programBState, has := programs.Get(contractBLocation)
		require.True(t, has)

		// program B should contain all the registers used by program A, as it depends on it
		require.IsType(t, programBState.View(), &delta.View{})
		deltaB := programBState.View().(*delta.View)

		idsA, valuesA := deltaA.Delta().RegisterUpdates()
		for i, id := range idsA {
			v, has := deltaB.Delta().Get(id.Owner, id.Controller, id.Key)
			require.True(t, has)

			require.Equal(t, valuesA[i], v)
		}

		for id, registerA := range deltaA.Interactions().Reads {

			registerB, has := deltaB.Interactions().Reads[id]
			require.True(t, has)

			require.Equal(t, registerA, registerB)
		}

		contractBView = deltaB

		// merge it back
		err = mainView.MergeView(viewExecB)
		require.NoError(t, err)

		// rerun transaction

		// execute transaction again, this time make sure it doesn't load code
		viewExecB2 := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			//this time we fail if a read of code occurs
			require.NotEqual(t, key, state.ContractKey("A"))
			require.NotEqual(t, key, state.ContractKey("B"))

			return mainView.Peek(owner, controller, key)
		})

		procCallB = fvm.Transaction(callTx("B", addressB), 5)

		err = vm.Run(context, procCallB, viewExecB2, programs)
		require.NoError(t, err)

		require.Contains(t, procCallB.Logs, "\"hello from B but also hello from A\"")

		compareViews(t, viewExecB, viewExecB2)

		// merge it back
		err = mainView.MergeView(viewExecB2)
		require.NoError(t, err)
	})

	t.Run("contract A runs from cache after program B has been loaded", func(t *testing.T) {

		// at this point programs cache should contain data for contract A
		// only because contract B has been called

		viewExecA := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			require.NotEqual(t, key, state.ContractKey("A"))
			return mainView.Peek(owner, controller, key)
		})

		// run a TX using contract A
		procCallA := fvm.Transaction(callTx("A", addressA), 6)

		err = vm.Run(context, procCallA, viewExecA, programs)
		require.NoError(t, err)

		require.Contains(t, procCallA.Logs, "\"hello from A\"")

		compareViews(t, txAView, viewExecA)

		// merge it back
		err = mainView.MergeView(viewExecA)
		require.NoError(t, err)
	})

	t.Run("deploying contract C cleans programs", func(t *testing.T) {
		require.NotNil(t, contractBView)

		// deploy contract C
		procContractC := fvm.Transaction(contractDeployTx("C", contractCCode, addressC), 7)
		err := vm.Run(context, procContractC, mainView, programs)
		require.NoError(t, err)

		_, _, hasA := programs.Get(contractALocation)
		_, _, hasB := programs.Get(contractBLocation)
		_, _, hasC := programs.Get(contractCLocation)

		require.False(t, hasA)
		require.False(t, hasB)
		require.False(t, hasC)

	})

	t.Run("importing C should chain-import B and A", func(t *testing.T) {
		procCallC := fvm.Transaction(callTx("C", addressC), 8)

		viewExecC := delta.NewView(mainView.Peek)

		err = vm.Run(context, procCallC, viewExecC, programs)
		require.NoError(t, err)

		require.Contains(t, procCallC.Logs, "\"hello from C, hello from B but also hello from A\"")

		// program A is the same
		_, programAState, has := programs.Get(contractALocation)
		require.True(t, has)

		require.IsType(t, programAState.View(), &delta.View{})
		deltaA := programAState.View().(*delta.View)
		compareViews(t, contractAView, deltaA)

		// program B is the same
		_, programBState, has := programs.Get(contractBLocation)
		require.True(t, has)

		require.IsType(t, programBState.View(), &delta.View{})
		deltaB := programBState.View().(*delta.View)
		compareViews(t, contractBView, deltaB)
	})
}

// compareViews compares views using only data that matters (ie. two different hasher instances
// trips the library comparison, even if actual SPoCKs are the same)
func compareViews(t *testing.T, a, b *delta.View) {
	require.Equal(t, a.Delta(), b.Delta())
	require.Equal(t, a.Interactions(), b.Interactions())
	require.Equal(t, a.ReadsCount(), b.ReadsCount())
	require.Equal(t, a.SpockSecret(), b.SpockSecret())
}
