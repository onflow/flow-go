package fvm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

const TopShotContractAddress = "0b2a3299cc857e29"

// Runtime is Cadence interface hence it would be hard to mock it
// implementation for our needs is simple enough though
type mockRuntime struct {
	executeTxResult error
}

func (m mockRuntime) ExecuteScript(script []byte, arguments [][]byte, runtimeInterface runtime.Interface, location runtime.Location) (cadence.Value, error) {
	panic("should not be used")
}

func (m mockRuntime) ExecuteTransaction(script []byte, arguments [][]byte, runtimeInterface runtime.Interface, location runtime.Location) error {
	return m.executeTxResult
}

func (m mockRuntime) ParseAndCheckProgram(code []byte, runtimeInterface runtime.Interface, location runtime.Location) (*sema.Checker, error) {
	panic("should not be used")
}

func prepare(executeTxResult error) (error, *bytes.Buffer) {

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)
	txInvocator := NewTransactionInvocator(log)

	runtime := mockRuntime{executeTxResult: executeTxResult}
	vm := New(runtime)

	proc := Transaction(&flow.TransactionBody{}, 0)

	ledger := state.NewMapLedger()

	context := NewContext(log)

	err := txInvocator.Process(vm, context, proc, ledger)

	return err, buffer
}

func TestTopShotSafety(t *testing.T) {

	t.Run("logs nothing for successful tx", func(t *testing.T) {
		err, buffer := prepare(nil)
		require.NoError(t, err)
		require.Equal(t, 0, buffer.Len())
	})

	t.Run("logs for failing tx but not related to TopShot", func(t *testing.T) {

		runtimeError := runtime.Error{
			Err: sema.CheckerError{
				Errors: []error{&sema.ImportedProgramError{
					CheckerError: &sema.CheckerError{},
					ImportLocation: ast.AddressLocation{
						Name:    "NotTopshot",
						Address: common.BytesToAddress([]byte{0, 1, 2}),
					},
				}},
			},
		}

		err, buffer := prepare(runtimeError)
		require.Error(t, err)
		require.Equal(t, 0, buffer.Len())
	})

	t.Run("logs for failing TopShot tx but no extra info is provided", func(t *testing.T) {

		topShotContract, err := hex.DecodeString(TopShotContractAddress)
		require.NoError(t, err)

		runtimeError := runtime.Error{
			Err: sema.CheckerError{
				Errors: []error{&sema.ImportedProgramError{
					CheckerError: &sema.CheckerError{},
					ImportLocation: ast.AddressLocation{
						Name:    "Topshot",
						Address: common.BytesToAddress(topShotContract),
					},
				}},
			},
		}

		err, buffer := prepare(runtimeError)
		require.Error(t, err)

		require.Contains(t, buffer.String(), "exception is not ExtendedParsingCheckingError")
	})

	t.Run("logs for failing TopShot tx", func(t *testing.T) {

		topShotContractAddress := flow.HexToAddress(TopShotContractAddress)

		runtimeError := runtime.Error{
			Err: &runtime.ParsingCheckingError{
				Err: sema.CheckerError{
					Errors: []error{&sema.ImportedProgramError{
						CheckerError: &sema.CheckerError{},
						ImportLocation: ast.AddressLocation{
							Name:    "TopShot",
							Address: common.BytesToAddress(topShotContractAddress.Bytes()),
						},
					}},
				},
				Code:     []byte("tx_code"),
				Location: nil,
				Options:  nil,
				UseCache: false,
				Checker:  nil,
			},
		}

		err, buffer := prepare(runtimeError)
		require.Error(t, err)

		expected := addressToSpewBytesString(topShotContractAddress)

		require.Contains(t, buffer.String(), expected)
	})

	t.Run("logs for failing TopShot tx in real environment", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `
			import TopShot from 0x0b2a3299cc857e29

			transaction {
				prepare(acct: AuthAccount) {
					let nftCollection = acct.borrow<&TopShot.Collection>(from: /storage/MomentCollection)
						?? panic("Could not borrow from MomentCollection in storage")
			
							
					nftCollection.deposit(token: <-token)
				}
			}
		`

		proc := Transaction(&flow.TransactionBody{
			ReferenceBlockID:   flow.Identifier{},
			Script:             []byte(code),
			Arguments:          nil,
			GasLimit:           0,
			ProposalKey:        flow.ProposalKey{},
			Payer:              flow.Address{},
			Authorizers:        nil,
			PayloadSignatures:  nil,
			EnvelopeSignatures: nil,
		}, 0)

		topShotContractAddress := flow.HexToAddress(TopShotContractAddress)

		ledger := state.NewMapLedger()

		topShotCode := `
			pub contract TopShot {
				 init() {
					self.currentSeries = 0
    			}
			}
		`

		encodedName, err := encodeContractNames([]string{"TopShot"})
		require.NoError(t, err)

		ledger.Set(string(topShotContractAddress.Bytes()), string(topShotContractAddress.Bytes()), "contract_names", encodedName)
		ledger.Set(string(topShotContractAddress.Bytes()), string(topShotContractAddress.Bytes()), "code.TopShot", []byte(topShotCode))

		context := NewContext(log)

		err = txInvocator.Process(vm, context, proc, ledger)
		require.Error(t, err)

		expected := addressToSpewBytesString(topShotContractAddress)

		require.Contains(t, buffer.String(), expected)
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

func addressToSpewBytesString(topShotContractAddress flow.Address) string {
	expectedBytes := topShotContractAddress.Bytes()

	stringBytes := make([]string, len(expectedBytes))

	for i, expectedByte := range expectedBytes {
		stringBytes[i] = fmt.Sprintf("%x", expectedByte)
	}

	expected := strings.Join(stringBytes, " ")
	return expected
}
