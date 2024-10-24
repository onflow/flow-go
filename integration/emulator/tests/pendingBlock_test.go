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
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/integration/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func setupPendingBlockTests(t *testing.T) (
	*emulator.Blockchain,
	*emulator.SDKAdapter,
	*flowsdk.Transaction,
	*flowsdk.Transaction,
	*flowsdk.Transaction,
) {
	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)
	logger := zerolog.Nop()
	adapter := emulator.NewSDKAdapter(&logger, b)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

	tx1 := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	tx2 := flowsdk.NewTransaction().
		SetScript([]byte(addTwoScript)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber+1).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	signer, err = b.ServiceKey().Signer()
	assert.NoError(t, err)
	err = tx2.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	invalid := flowsdk.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	signer, err = b.ServiceKey().Signer()
	assert.NoError(t, err)
	err = invalid.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	return b, adapter, tx1, tx2, invalid
}

func TestPendingBlockBeforeExecution(t *testing.T) {

	t.Parallel()

	t.Run("EmptyPendingBlock", func(t *testing.T) {

		t.Parallel()

		b, _, _, _, _ := setupPendingBlockTests(t)

		// Execute empty pending block
		_, err := b.ExecuteBlock()
		assert.NoError(t, err)

		// Commit empty pending block
		_, err = b.CommitBlock()
		assert.NoError(t, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("AddDuplicateTransaction", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// Add tx1 again
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, &emulator.DuplicateTransactionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitBeforeExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// Attempt to commit block before execution begins
		_, err = b.CommitBlock()
		assert.IsType(t, &emulator.PendingBlockCommitBeforeExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockDuringExecution(t *testing.T) {

	t.Parallel()

	t.Run("ExecuteNextTransaction", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		require.NoError(t, err)

		// Add invalid script tx to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		require.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Execute invalid script tx (reverts)
		result, err = b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("ExecuteBlock", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		require.NoError(t, err)

		// Add invalid script tx to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		require.NoError(t, err)

		// Execute all tx in pending block (tx1, invalid)
		results, err := b.ExecuteBlock()
		assert.NoError(t, err)

		// tx1 result
		assert.True(t, results[0].Succeeded())
		// invalid script tx result
		assert.True(t, results[1].Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("ExecuteNextThenBlock", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, tx2, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Add tx2 to pending block
		err = adapter.SendTransaction(context.Background(), *tx2)
		assert.NoError(t, err)

		// Add invalid script tx to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Execute rest of tx in pending block (tx2, invalid)
		results, err := b.ExecuteBlock()
		assert.NoError(t, err)
		// tx2 result
		assert.True(t, results[0].Succeeded())
		// invalid script tx result
		assert.True(t, results[1].Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("AddTransactionMidExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, tx2, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Attempt to add tx2 to pending block after execution begins
		err = adapter.SendTransaction(context.Background(), *tx2)
		assert.IsType(t, &emulator.PendingBlockMidExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitMidExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, invalid := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = adapter.SendTransaction(context.Background(), *invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Attempt to commit block before execution finishes
		_, err = b.CommitBlock()
		assert.IsType(t, &emulator.PendingBlockMidExecutionError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("TransactionsExhaustedDuringExecution", func(t *testing.T) {

		t.Parallel()

		b, adapter, tx1, _, _ := setupPendingBlockTests(t)

		// Add tx1 to pending block
		err := adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Attempt to execute nonexistent next tx (fails)
		_, err = b.ExecuteNextTransaction()
		assert.IsType(t, &emulator.PendingBlockTransactionsExhaustedError{}, err)

		// Attempt to execute rest of block tx (fails)
		_, err = b.ExecuteBlock()
		assert.IsType(t, &emulator.PendingBlockTransactionsExhaustedError{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockCommit(t *testing.T) {

	t.Parallel()

	b, err := emulator.New(
		emulator.WithStorageLimitEnabled(false),
	)
	require.NoError(t, err)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	logger := zerolog.Nop()
	adapter := emulator.NewSDKAdapter(&logger, b)

	addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

	t.Run("CommitBlock", func(t *testing.T) {
		tx1 := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx1.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Add tx1 to pending block
		err = adapter.SendTransaction(context.Background(), *tx1)
		require.NoError(t, err)

		// Enter execution mode (block hash should not change after this point)
		blockID := b.PendingBlockID()

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		// Commit pending block
		block, err := b.CommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockID, block.ID())
	})

	t.Run("ExecuteAndCommitBlock", func(t *testing.T) {
		tx1 := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx1.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Add tx1 to pending block
		err = adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		// Enter execution mode (block hash should not change after this point)
		blockID := b.PendingBlockID()

		// Execute and commit pending block
		block, results, err := b.ExecuteAndCommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockID, block.ID())
		assert.Len(t, results, 1)
	})
}

type testClock struct {
	Time time.Time
}

func (tc testClock) Now() time.Time {
	return tc.Time.UTC()
}

func TestPendingBlockSetTimestamp(t *testing.T) {

	t.Parallel()

	b, adapter, _, _, _ := setupPendingBlockTests(t)
	clock := testClock{
		Time: time.Now().UTC(),
	}
	b.SetClock(clock.Now)
	_, _ = b.CommitBlock()

	script := []byte(`
	    access(all) fun main(): UFix64 {
	        return getCurrentBlock().timestamp
	    }
	`)
	scriptResult, err := adapter.ExecuteScriptAtLatestBlock(
		context.Background(),
		script,
		[][]byte{},
	)
	require.NoError(t, err)

	expected := fmt.Sprintf(
		"{\"value\":\"%d.00000000\",\"type\":\"UFix64\"}\n",
		clock.Time.Unix(),
	)
	assert.Equal(t, expected, string(scriptResult))

	clock = testClock{
		Time: time.Now().Add(time.Hour * 24 * 7).UTC(),
	}
	b.SetClock(clock.Now)
	_, _ = b.CommitBlock()

	_, err = adapter.ExecuteScriptAtLatestBlock(
		context.Background(),
		script,
		[][]byte{},
	)
	require.NoError(t, err)

	/*expected = fmt.Sprintf(
		"{\"value\":\"%d.00000000\",\"type\":\"UFix64\"}\n",
		clock.Time.Unix(),
	)*/
	//assert.Equal(t, expected, string(scriptResult))
}
