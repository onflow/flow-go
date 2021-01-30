package fvm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func TestSafetyCheck(t *testing.T) {

	t.Run("parsing error in imported contract", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `
			import 0x0b2a3299cc857e29

			transaction {}
		`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		contractAddress := flow.HexToAddress("0b2a3299cc857e29")

		ledger := state.NewMapLedger()

		contractCode := `X`

		// TODO: refactor this manual deployment by setting ledger keys
		//   into a proper deployment of the contract

		encodedName, err := encodeContractNames([]string{"TestContract"})
		require.NoError(t, err)

		err = ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"contract_names",
			encodedName,
		)
		require.NoError(t, err)
		err = ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"code.TestContract",
			[]byte(contractCode),
		)
		require.NoError(t, err)

		context := NewContext(log)

		st := state.NewState(
			ledger,
			state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
			state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
			state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
		)

		err = txInvocator.Process(vm, context, proc, st)
		require.Error(t, err)

		require.Contains(t, buffer.String(), "programs")
		require.Contains(t, buffer.String(), "codes")
	})

	t.Run("checking error in imported contract", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `
			import 0x0b2a3299cc857e29

			transaction {}
		`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		contractAddress := flow.HexToAddress("0b2a3299cc857e29")

		ledger := state.NewMapLedger()

		contractCode := `pub contract TestContract: X {}`

		// TODO: refactor this manual deployment by setting ledger keys
		//   into a proper deployment of the contract

		encodedName, err := encodeContractNames([]string{"TestContract"})
		require.NoError(t, err)

		err = ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"contract_names",
			encodedName,
		)
		require.NoError(t, err)
		err = ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"code.TestContract",
			[]byte(contractCode),
		)
		require.NoError(t, err)

		context := NewContext(log)

		st := state.NewState(
			ledger,
			state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
			state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
			state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
		)

		err = txInvocator.Process(vm, context, proc, st)
		require.Error(t, err)

		require.Contains(t, buffer.String(), "programs")
		require.Contains(t, buffer.String(), "codes")
	})

	t.Run("parsing error in transaction", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `X`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		ledger := state.NewMapLedger()
		context := NewContext(log)

		st := state.NewState(
			ledger,
			state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
			state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
			state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
		)

		err := txInvocator.Process(vm, context, proc, st)
		require.Error(t, err)

		require.NotContains(t, buffer.String(), "programs")
		require.NotContains(t, buffer.String(), "codes")
	})

	t.Run("checking error in transaction", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `transaction(arg: X) { }`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		ledger := state.NewMapLedger()
		context := NewContext(log)

		st := state.NewState(
			ledger,
			state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
			state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
			state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
		)

		err := txInvocator.Process(vm, context, proc, st)
		require.Error(t, err)

		require.NotContains(t, buffer.String(), "programs")
		require.NotContains(t, buffer.String(), "codes")
	})
}

func TestFailedTransactionsDontModifyProgramsCache(t *testing.T) {

	ledger := state.NewMapLedger()
	st := state.NewState(ledger)

	address := flow.HexToAddress("1234")

	//create accounts
	accounts := state.NewAccounts(st)
	err := accounts.Create(nil, address)
	require.NoError(t, err)

	err = st.Commit()
	require.NoError(t, err)

	contractName := "Cherry"

	contractACode := fmt.Sprintf(`
			pub contract %s {
				pub fun say() {
					log("a")
				}
			}
		`, contractName)

	contractBCode := fmt.Sprintf(`
			pub contract %s {
				pub fun shout() {
					log("a")
				}
			}
		`, contractName)

	deployContract := func(contractName, contractCode string) []byte {
		return []byte(fmt.Sprintf(
			`
			 transaction {
			   prepare(signer: AuthAccount) {
				   signer.contracts.add(name: "%s", code: "%s".decodeHex())
			   }
			 }
	   `, contractName, hex.EncodeToString([]byte(contractCode)),
		))
	}

	updateContract := func(contractName, contractCode string) []byte {
		return []byte(fmt.Sprintf(
			`
			 transaction {
			   prepare(signer: AuthAccount) {
				   signer.contracts.update__experimental(name: "%s", code: "%s".decodeHex())
			   }
			 }
	   `, contractName, hex.EncodeToString([]byte(contractCode)),
		))
	}

	procA := Transaction(&flow.TransactionBody{Script: deployContract(contractName, contractACode), Authorizers: []flow.Address{address}, Payer: address}, 0)
	deployTxInvocator := NewTransactionInvocator(zerolog.Nop())
	deployContext := NewContext(zerolog.Nop(), WithTransactionProcessors(deployTxInvocator), WithRestrictedDeployment(false))
	deployRt := runtime.NewInterpreterRuntime()

	deployVm := New(deployRt)

	err = deployVm.Run(deployContext, procA, st)
	require.NoError(t, err)
	require.NoError(t, procA.Err)

	programBefore := deployContext.Programs.Get(common.LocationID(fmt.Sprintf("A.%s.%s", address, contractName)))
	require.NotNil(t, programBefore) //make sure we get some real program

	procB := Transaction(&flow.TransactionBody{Script: updateContract(contractName, contractBCode), Authorizers: []flow.Address{address}, Payer: address}, 0)

	deployContextButFailing := NewContextFromParent(deployContext, WithTransactionProcessors(deployTxInvocator, &AlwaysFailingTransactionProcessor{}))
	err = deployVm.Run(deployContextButFailing, procB, st)
	require.NoError(t, err)
	require.Error(t, procB.Err)

	programAfter := deployContextButFailing.Programs.Get(common.LocationID(fmt.Sprintf("A.%s.%s", address, contractName)))

	require.Equal(t, programBefore, programAfter)
}

type AlwaysFailingTransactionProcessor struct{}

func (a *AlwaysFailingTransactionProcessor) Process(*VirtualMachine, Context, *TransactionProcedure, *state.State) error {
	return &FvmError{}
}

type FvmError struct{}

func (f *FvmError) Code() uint32 {
	return 6
}

func (f *FvmError) Error() string {
	return "I am FVM error"
}

func encodeContractNames(contractNames []string) ([]byte, error) {
	sort.Strings(contractNames)
	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err := cborEncoder.Encode(contractNames)
	if err != nil {
		return nil, fmt.Errorf("cannot encode contract names")
	}
	return buf.Bytes(), nil
}
