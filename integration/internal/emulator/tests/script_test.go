/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flowsdk "github.com/onflow/flow-go-sdk"

	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	emulator "github.com/onflow/flow-go/integration/internal/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func TestExecuteScript(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	logger := zerolog.Nop()
	adapter := emulator.NewSDKAdapter(&logger, b)

	addTwoScript, counterAddress := DeployAndGenerateAddTwoScript(t, adapter)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	callScript := GenerateGetCounterCountScript(counterAddress, serviceAccountAddress)

	// Sample call (value is 0)
	scriptResult, err := b.ExecuteScript([]byte(callScript), nil)
	require.NoError(t, err)
	assert.Equal(t, cadence.NewInt(0), scriptResult.Value)

	// Submit tx (script adds 2)
	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	txResult, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, txResult)

	t.Run("BeforeCommit", func(t *testing.T) {
		t.Skip("TODO: fix stored ledger")

		// Sample call (value is still 0)
		result, err := b.ExecuteScript([]byte(callScript), nil)
		require.NoError(t, err)
		assert.Equal(t, cadence.NewInt(0), result.Value)
	})

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	t.Run("AfterCommit", func(t *testing.T) {
		// Sample call (value is 2)
		result, err := b.ExecuteScript([]byte(callScript), nil)
		require.NoError(t, err)
		assert.Equal(t, cadence.NewInt(2), result.Value)
	})
}

func TestExecuteScript_WithArguments(t *testing.T) {

	t.Parallel()

	t.Run("Int", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.New()
		require.NoError(t, err)

		scriptWithArgs := `
			access(all) fun main(n: Int): Int {
				return n
			}
		`

		arg, err := jsoncdc.Encode(cadence.NewInt(10))
		require.NoError(t, err)

		scriptResult, err := b.ExecuteScript([]byte(scriptWithArgs), [][]byte{arg})
		require.NoError(t, err)

		assert.Equal(t, cadence.NewInt(10), scriptResult.Value)
	})

	t.Run("String", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.New()
		require.NoError(t, err)

		scriptWithArgs := `
			access(all) fun main(n: String): Int {
				log(n)
				return 0
			}
		`

		arg, err := jsoncdc.Encode(cadence.String("Hello, World"))
		require.NoError(t, err)
		scriptResult, err := b.ExecuteScript([]byte(scriptWithArgs), [][]byte{arg})
		require.NoError(t, err)
		assert.Contains(t, scriptResult.Logs, "\"Hello, World\"")
	})
}

func TestExecuteScript_FlowServiceAccountBalance(t *testing.T) {

	t.Parallel()

	b, err := emulator.New()
	require.NoError(t, err)

	code := fmt.Sprintf(
		`
          import FlowServiceAccount from %[1]s

          access(all)
          fun main(): UFix64 {
              let acct = getAccount(%[1]s)
              return FlowServiceAccount.defaultTokenBalance(acct)
          }
        `,
		b.GetChain().ServiceAddress().HexWithPrefix(),
	)

	res, err := b.ExecuteScript([]byte(code), nil)
	require.NoError(t, err)
	require.NoError(t, res.Error)

	require.Positive(t, res.Value)
}

func TestInfiniteScript(t *testing.T) {

	t.Parallel()

	const limit = 18
	b, err := emulator.New(
		emulator.WithScriptGasLimit(limit),
	)
	require.NoError(t, err)

	const code = `
		access(all) fun main() {
			main()
		}
	 `
	result, err := b.ExecuteScript([]byte(code), nil)
	require.NoError(t, err)

	require.True(t, fvmerrors.IsComputationLimitExceededError(result.Error))
}

func TestScriptExecutionLimit(t *testing.T) {

	t.Parallel()

	const code = `
		access(all) fun main() {
			var s: Int256 = 1024102410241024
			var i: Int256 = 0
			var a: Int256 = 7
			var b: Int256 = 5
			var c: Int256 = 2

			while i < 150000 {
				s = s * a
				s = s / b
				s = s / c
				i = i + 1
			}
		}
	 `

	t.Run("ExceedingLimit", func(t *testing.T) {

		t.Parallel()

		const limit = 2000
		b, err := emulator.New(
			emulator.WithScriptGasLimit(limit),
		)
		require.NoError(t, err)

		result, err := b.ExecuteScript([]byte(code), nil)
		require.NoError(t, err)

		require.True(t, fvmerrors.IsComputationLimitExceededError(result.Error))
	})

	t.Run("SufficientLimit", func(t *testing.T) {

		t.Parallel()

		const limit = 19000
		b, err := emulator.New(
			emulator.WithScriptGasLimit(limit),
		)
		require.NoError(t, err)

		result, err := b.ExecuteScript([]byte(code), nil)
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

// TestScriptWithCadenceRandom checks Cadence's random function works
// within a script
func TestScriptWithCadenceRandom(t *testing.T) {

	//language=cadence
	code := `
      access(all)
      fun main() {
          assert(revertibleRandom<UInt64>() >= 0)
      }
	`

	const limit = 200
	b, err := emulator.New(
		emulator.WithScriptGasLimit(limit),
	)
	require.NoError(t, err)

	result, err := b.ExecuteScript([]byte(code), nil)
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

// TestEVM checks evm functionality
func TestEVM(t *testing.T) {
	serviceAddr := flowgo.Emulator.Chain().ServiceAddress()
	code := []byte(fmt.Sprintf(
		`
			import EVM from 0x%s

			access(all)
			fun main(bytes: [UInt8; 20]) {
				log(EVM.EVMAddress(bytes: bytes))
			}
		`,
		serviceAddr,
	))

	gasLimit := uint64(100_000)

	b, err := emulator.New(
		emulator.WithScriptGasLimit(gasLimit),
	)
	require.NoError(t, err)

	addressBytesArray := cadence.NewArray([]cadence.Value{
		cadence.UInt8(1), cadence.UInt8(1),
		cadence.UInt8(2), cadence.UInt8(2),
		cadence.UInt8(3), cadence.UInt8(3),
		cadence.UInt8(4), cadence.UInt8(4),
		cadence.UInt8(5), cadence.UInt8(5),
		cadence.UInt8(6), cadence.UInt8(6),
		cadence.UInt8(7), cadence.UInt8(7),
		cadence.UInt8(8), cadence.UInt8(8),
		cadence.UInt8(9), cadence.UInt8(9),
		cadence.UInt8(10), cadence.UInt8(10),
	}).WithType(stdlib.EVMAddressBytesCadenceType)

	result, err := b.ExecuteScript(code, [][]byte{jsoncdc.MustEncode(addressBytesArray)})
	require.NoError(t, err)
	require.NoError(t, result.Error)
	require.Len(t, result.Logs, 1)
	require.Equal(t, result.Logs[0], fmt.Sprintf("A.%s.EVM.EVMAddress(bytes: %s)", serviceAddr, addressBytesArray.String()))

}
