package handler_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
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
	//addressC := flow.HexToAddress("c")
	//addressD := flow.HexToAddress("d")

	contractALocation := common.AddressLocation{
		Address: common.BytesToAddress(addressA.Bytes()),
		Name:    "A",
	}

	contractBLocation := common.AddressLocation{
		Address: common.BytesToAddress(addressB.Bytes()),
		Name:    "B",
	}

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
       		return "hello from B but also".concat(A.hello())
    		}
		}
	`
	//
	//contractCCode := `
	//	import A from 0xa
	//	import B from 0xb
	//
	//	pub contract C {
	//		pub fun hello(): String {
	//    		return "hello from C, from B".concat(B.hello()).concat("but also from A").concat(A.hello())
	//		}
	//	}
	//`

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

	mainView := delta.NewView(func(_, _, _ string) (flow.RegisterValue, error) {
		return nil, nil
	})

	stm := state.NewStateManager(state.NewState(mainView))

	rt := runtime.NewInterpreterRuntime()
	vm := fvm.New(rt)
	programs := programsStorage.NewEmptyPrograms()

	accounts := state.NewAccounts(stm)

	err := accounts.Create(nil, addressA)
	require.NoError(t, err)

	err = accounts.Create(nil, addressB)
	require.NoError(t, err)

	err = stm.ApplyStartStateToLedger()
	require.NoError(t, err)

	context := fvm.NewContext(zerolog.Nop(), fvm.WithRestrictedDeployment(false), fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(zerolog.Nop())), fvm.WithCadenceLogging(true))

	var contractALedger state.Ledger = nil

	t.Run("register touches are captured for simple contract A", func(t *testing.T) {

		// deploy contract A
		procContractA := fvm.Transaction(contractDeployTx("A", contractACode, addressA), 0)
		err := vm.Run(context, procContractA, mainView, programs)
		require.NoError(t, err)

		//buffer := &bytes.Buffer{}
		//log := zerolog.New(buffer)

		// run a TX using contract A
		procCallA := fvm.Transaction(callTx("A", addressA), 1)

		view := mainView.NewChild()

		err = vm.Run(context, procCallA, view, programs)
		require.NoError(t, err)

		_, programState, has := programs.Get(contractALocation)
		require.True(t, has)
		// recorded state should be equal to fresh one created for this TX (as there are no other operation in a TX)
		require.Equal(t, programState.Ledger(), view)

		contractALedger = programState.Ledger()

		// merge it back
		mainView.MergeView(view)
	})

	t.Run("deploying another contract cleans programs storage", func(t *testing.T) {

		// deploy contract B
		procContractB := fvm.Transaction(contractDeployTx("B", contractBCode, addressB), 2)
		err := vm.Run(context, procContractB, mainView, programs)
		require.NoError(t, err)

		_, _, hasA := programs.Get(contractALocation)
		_, _, hasB := programs.Get(contractBLocation)

		require.False(t, hasA)
		require.False(t, hasB)
	})

	var viewB *delta.View

	t.Run("contract imports other contracts", func(t *testing.T) {

		// run a TX using contract B
		procCallB := fvm.Transaction(callTx("B", addressB), 3)

		viewB = mainView.NewChild()

		err = vm.Run(context, procCallB, viewB, programs)
		require.NoError(t, err)

		_, programAState, has := programs.Get(contractALocation)
		require.True(t, has)
		// state should be essentially the same as one which we got in tx with contract A
		require.Equal(t, contractALedger, programAState.Ledger())

		_, programBState, has := programs.Get(contractBLocation)
		require.True(t, has)
		// recorded state should be equal to fresh one created for this TX (as there are no other operation in a TX)
		require.Equal(t, programBState.Ledger(), viewB)

		// merge it back
		mainView.MergeView(viewB)
	})

	t.Run("running same transaction should result exactly the same touches", func(t *testing.T) {

		// run a TX using contract B
		procCallB := fvm.Transaction(callTx("B", addressB), 4)

		view := mainView.NewChild()

		err = vm.Run(context, procCallB, viewB, programs)
		require.NoError(t, err)

		//
		require.Equal(t, viewB, view)
	})

}
