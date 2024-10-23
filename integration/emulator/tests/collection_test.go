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
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/integration/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func TestCollections(t *testing.T) {

	t.Parallel()

	t.Run("Empty block", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.New()
		require.NoError(t, err)

		block, err := b.CommitBlock()
		require.NoError(t, err)

		// block should not contain any collections
		assert.Empty(t, block.Payload.Guarantees)
	})

	t.Run("Non-empty block", func(t *testing.T) {

		t.Parallel()

		b, err := emulator.New(
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		require.NoError(t, err)

		logger := zerolog.Nop()
		adapter := emulator.NewSDKAdapter(&logger, b)

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
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		err = tx2.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// generate a list of transactions
		transactions := []*flowsdk.Transaction{tx1, tx2}

		// add all transactions to block
		for _, tx := range transactions {
			err = adapter.SendTransaction(context.Background(), *tx)
			require.NoError(t, err)
		}

		block, _, err := b.ExecuteAndCommitBlock()
		require.NoError(t, err)

		// block should contain at least one collection
		assert.NotEmpty(t, block.Payload.Guarantees)

		i := 0
		for _, guarantee := range block.Payload.Guarantees {
			collection, err := adapter.GetCollectionByID(context.Background(), emulator.FlowIdentifierToSDK(guarantee.ID()))
			require.NoError(t, err)

			for _, txID := range collection.TransactionIDs {
				assert.Equal(t, transactions[i].ID(), txID)
				i++
			}
		}
	})
}
