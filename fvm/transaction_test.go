package fvm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence/runtime"
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

func TestAccountFreezing(t *testing.T) {

	ledger := state.NewMapLedger()
	st := state.NewState(ledger)

	//frozenAddress := flow.HexToAddress("1234")
	notFrozenAddress := flow.HexToAddress("5678")

	accounts := state.NewAccounts(st)
	//err := accounts.Create(nil, frozenAddress)
	//require.NoError(t, err)
	err := accounts.Create(nil, notFrozenAddress)
	require.NoError(t, err)

	//err = accounts.SetAccountFrozen(frozenAddress, true)
	//require.NoError(t, err)

	whateverContractCode := `
		pub contract Whatever {
			pub fun say() {
				log("whatever not frozen")
			}
		}
	`
	//err = accounts.SetContract("Whatever", notFrozenAddress, []byte(whateverContractCode))
	//require.NoError(t, err)

	err = st.Commit()
	require.NoError(t, err)

	t.Run("code from frozen account cannot be loaded", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := fmt.Sprintf(`
			import Whatever from 0x%s

			transaction {
				execute {
					Whatever.say()
				}
			}
		`, notFrozenAddress.String())

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		context := NewContext(log)

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)
		require.Contains(t, buffer, "whatever not frozen")
	})

	t.Run("set code should set code", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		deployContract := []byte(fmt.Sprintf(
			`
          transaction {
            prepare(signer: AuthAccount) {
                let acct = AuthAccount(payer: signer)
                acct.contracts.add(name: "Whatever", code: "%s".decodeHex())
            }
          }
        `,
			hex.EncodeToString([]byte(whateverContractCode)),
		))

		proc := Transaction(&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{notFrozenAddress}, Payer: notFrozenAddress}, 0)

		context := NewContext(log, WithServiceAccount(false), WithRestrictedDeployment(false))

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)

		err = st.Commit()
		require.NoError(t, err)

		codeB := fmt.Sprintf(`
			import Whatever from 0x%s

			transaction {
				execute {
					Whatever.say()
				}
			}
		`, notFrozenAddress.String())

		proc = Transaction(&flow.TransactionBody{Script: []byte(codeB)}, 0)

		context = NewContext(log)

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)
		require.Contains(t, buffer, "whatever not frozen")
	})

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
