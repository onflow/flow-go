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

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flowsdk "github.com/onflow/flow-go-sdk"

	emulator "github.com/onflow/flow-go/integration/internal/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func TestCommitBlock(t *testing.T) {

	t.Parallel()

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

	// Add tx1 to pending block
	err = adapter.SendTransaction(context.Background(), *tx1)
	assert.NoError(t, err)

	tx1Result, err := adapter.GetTransactionResult(context.Background(), tx1.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusPending, tx1Result.Status)

	tx2 := flowsdk.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	signer, err = b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx2.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Add tx2 to pending block
	err = adapter.SendTransaction(context.Background(), *tx2)
	require.NoError(t, err)

	tx2Result, err := adapter.GetTransactionResult(context.Background(), tx2.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusPending, tx2Result.Status)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	// Execute tx2
	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Reverted())

	// Commit tx1 and tx2 into new block
	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err = adapter.GetTransactionResult(context.Background(), tx1.ID())
	require.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)

	// tx2 status also becomes TransactionStatusSealed, even though it is reverted
	tx2Result, err = adapter.GetTransactionResult(context.Background(), tx2.ID())
	require.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, tx2Result.Status)
	assert.Error(t, tx2Result.Error)
}

func TestBlockView(t *testing.T) {

	t.Parallel()

	const nBlocks = 3

	b, err := emulator.New()
	require.NoError(t, err)

	logger := zerolog.Nop()
	adapter := emulator.NewSDKAdapter(&logger, b)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	t.Run("genesis should have 0 view", func(t *testing.T) {
		block, err := b.GetBlockByHeight(0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), block.Header.Height)
		assert.Equal(t, uint64(0), block.Header.View)
	})

	addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

	// create a few blocks, each with one transaction
	for i := 0; i < nBlocks; i++ {

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

		// Add tx to pending block
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// execute and commit the block
		_, _, err = b.ExecuteAndCommitBlock()
		require.NoError(t, err)
	}
	const MaxViewIncrease = 3

	for height := uint64(1); height <= nBlocks+1; height++ {
		block, err := b.GetBlockByHeight(height)
		require.NoError(t, err)

		maxView := height * MaxViewIncrease
		t.Run(fmt.Sprintf("block %d should have view <%d", height, maxView), func(t *testing.T) {
			assert.Equal(t, height, block.Header.Height)
			assert.LessOrEqual(t, block.Header.View, maxView)
		})
	}
}
