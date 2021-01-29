package fvm

import (
	"bytes"
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
