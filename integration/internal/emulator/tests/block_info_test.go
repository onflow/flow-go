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

func TestBlockInfo(t *testing.T) {

	t.Parallel()

	b, err := emulator.New()
	require.NoError(t, err)

	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	logger := zerolog.Nop()
	adapter := emulator.NewSDKAdapter(&logger, b)

	block1, err := b.CommitBlock()
	require.NoError(t, err)

	block2, err := b.CommitBlock()
	require.NoError(t, err)

	t.Run("works as transaction", func(t *testing.T) {
		tx := flowsdk.NewTransaction().
			SetScript([]byte(`
				transaction {
					execute {
						let block = getCurrentBlock()
						log(block)

						let lastBlock = getBlock(at: block.height - 1)
						log(lastBlock)
					}
				}
			`)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, view: %v, id: 0x%x, timestamp: %.3f)", block2.Header.Height+1,
			b.PendingBlockView(), b.PendingBlockID(), float64(b.PendingBlockTimestamp())/1000), result.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, view: %v, id: 0x%x, timestamp: %.3f)", block2.Header.Height,
			block2.Header.View, block2.ID(), float64(block2.Header.Timestamp)/1000), result.Logs[1])
	})

	t.Run("works as script", func(t *testing.T) {
		script := []byte(`
			access(all) fun main() {
				let block = getCurrentBlock()
				log(block)

				let lastBlock = getBlock(at: block.height - 1)
				log(lastBlock)
			}
		`)

		result, err := b.ExecuteScript(script, nil)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())

		require.Len(t, result.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, view: %v, id: 0x%x, timestamp: %.3f)", block2.Header.Height,
			block2.Header.View, block2.ID(), float64(block2.Header.Timestamp)/1000), result.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, view: %v, id: 0x%x, timestamp: %.3f)", block1.Header.Height,
			block1.Header.View, block1.ID(), float64(block1.Header.Timestamp)/1000), result.Logs[1])
	})
}
