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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"

	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	emulator "github.com/onflow/flow-go/integration/internal/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func setupTransactionTests(t *testing.T, opts ...emulator.Option) (
	*emulator.Blockchain,
	*emulator.SDKAdapter,
) {
	b, err := emulator.New(opts...)
	require.NoError(t, err)

	logger := zerolog.Nop()
	return b, emulator.NewSDKAdapter(&logger, b)
}

func TestSubmitTransaction(t *testing.T) {

	t.Parallel()

	b, adapter := setupTransactionTests(t)
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

	// Submit tx1
	err = adapter.SendTransaction(context.Background(), *tx1)
	assert.NoError(t, err)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err := adapter.GetTransactionResult(context.Background(), tx1.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)
}

// TODO: Add test case for missing ReferenceBlockID
// TODO: Add test case for missing ProposalKey
func TestSubmitTransaction_Invalid(t *testing.T) {

	t.Parallel()

	t.Run("Empty transaction", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		// Create empty transaction (no required fields)
		tx := flowsdk.NewTransaction()

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing script", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		// Create transaction with no Script field
		tx := flowsdk.NewTransaction().
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Invalid script", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		// Create transaction with invalid Script field
		tx := flowsdk.NewTransaction().
			SetScript([]byte("this script cannot be parsed")).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, &emulator.InvalidTransactionScriptError{}, err)
	})

	t.Run("Missing gas limit", func(t *testing.T) {

		t.Parallel()

		t.Skip("TODO: transaction validation")

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

		// Create transaction with no GasLimit field
		tx := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Missing payer account", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

		// Create transaction with no PayerAccount field
		tx := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, err, &emulator.IncompleteTransactionError{})
	})

	t.Run("Missing proposal key", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

		// Create transaction with no PayerAccount field
		tx := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit)

		tx.ProposalKey = flowsdk.ProposalKey{}

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Invalid sequence number", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

		invalidSequenceNumber := b.ServiceKey().SequenceNumber + 2137
		tx := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetPayer(serviceAccountAddress).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, invalidSequenceNumber).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			AddAuthorizer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)

		require.Error(t, result.Error)

		assert.IsType(t, &emulator.FVMError{}, result.Error)
		seqErr := fvmerrors.InvalidProposalSeqNumberError{}
		ok := errors.As(result.Error, &seqErr)
		assert.True(t, ok)
		assert.Equal(t, invalidSequenceNumber, seqErr.ProvidedSeqNumber())
	})

	const expiry = 10

	t.Run("Missing reference block ID", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithTransactionExpiry(expiry),
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

		tx := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, &emulator.IncompleteTransactionError{}, err)
	})

	t.Run("Expired transaction", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithTransactionExpiry(expiry),
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

		expiredBlock, err := b.GetLatestBlock()
		require.NoError(t, err)

		// commit blocks until expiry window is exceeded
		for i := 0; i < expiry+1; i++ {
			_, _, err := b.ExecuteAndCommitBlock()
			require.NoError(t, err)
		}

		tx := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetReferenceBlockID(flowsdk.Identifier(expiredBlock.ID())).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.IsType(t, &emulator.ExpiredTransactionError{}, err)
	})

	t.Run("Invalid signature for provided data", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

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

		tx.SetComputeLimit(100) // change data after signing

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		debug := emulator.NewTransactionInvalidSignature(&flowgo.TransactionBody{
			ReferenceBlockID: flowgo.Identifier{},
			Script:           nil,
			Arguments:        nil,
			GasLimit:         flowgo.DefaultMaxTransactionGasLimit,
			ProposalKey: flowgo.ProposalKey{
				Address:        emulator.SDKAddressToFlow(serviceAccountAddress),
				KeyIndex:       b.ServiceKey().Index,
				SequenceNumber: b.ServiceKey().SequenceNumber,
			},
			Payer:              emulator.SDKAddressToFlow(serviceAccountAddress),
			Authorizers:        emulator.SDKAddressesToFlow([]flowsdk.Address{serviceAccountAddress}),
			PayloadSignatures:  nil,
			EnvelopeSignatures: nil,
		})

		assert.NotNil(t, result.Error)
		assert.IsType(t, result.Debug, debug)
	})
}

func TestSubmitTransaction_Duplicate(t *testing.T) {

	t.Parallel()

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
	)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

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

	// Submit tx
	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// Submit same tx again (errors)
	err = adapter.SendTransaction(context.Background(), *tx)
	assert.IsType(t, err, &emulator.DuplicateTransactionError{})
}

func TestSubmitTransaction_Reverted(t *testing.T) {

	t.Parallel()

	b, adapter := setupTransactionTests(t)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(`transaction { execute { panic("revert!") } }`)).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress).
		AddAuthorizer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// Submit invalid tx1
	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Reverted())

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err := adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)
	assert.Error(t, tx1Result.Error)
}

func TestSubmitTransaction_Authorizers(t *testing.T) {

	t.Parallel()

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
	)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	accountKeys := test.AccountKeyGenerator()

	accountKeyB, signerB := accountKeys.NewWithSigner()
	accountKeyB.SetWeight(flowsdk.AccountKeyWeightThreshold)

	accountAddressB, err := adapter.CreateAccount(context.Background(), []*flowsdk.AccountKey{accountKeyB}, nil)
	assert.NoError(t, err)

	t.Run("Extra authorizers", func(t *testing.T) {
		// script only supports one account
		script := []byte(`
		  transaction {
		    prepare(signer: &Account) {}
		  }
		`)

		// create transaction with two authorizing accounts
		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		assert.True(t, result.Reverted())

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})

	t.Run("Insufficient authorizers", func(t *testing.T) {
		// script requires two accounts
		script := []byte(`
		  transaction {
		    prepare(signerA: &Account, signerB: &Account) {}
		  }
		`)

		// create transaction with two accounts
		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})
}

func TestSubmitTransaction_EnvelopeSignature(t *testing.T) {

	t.Parallel()

	t.Run("Invalid account", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addresses := flowsdk.NewAddressGenerator(flowsdk.Emulator)
		for {
			_, err := adapter.GetAccount(context.Background(), addresses.NextAddress())
			if err != nil {
				break
			}
		}

		nonExistentAccountAddress := addresses.Address()

		script := []byte(`
		  transaction {
		    prepare(signer: &Account) {}
		  }
		`)

		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(nonExistentAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignPayload(nonExistentAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.Error(t, result.Error)
		assert.True(t, fvmerrors.IsAccountPublicKeyNotFoundError(result.Error))
	})

	t.Run("Mismatched authorizer count", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithTransactionValidationEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addresses := flowsdk.NewAddressGenerator(flowsdk.Emulator)
		for {
			_, err := adapter.GetAccount(context.Background(), addresses.NextAddress())
			if err != nil {
				break
			}
		}

		nonExistentAccountAddress := addresses.Address()

		script := []byte(`
		  transaction {
		    prepare() {}
		  }
		`)

		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(nonExistentAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignPayload(nonExistentAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.ErrorContains(t, result.Error, "authorizer count mismatch")
	})

	t.Run("Invalid key", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		addTwoScript, _ := DeployAndGenerateAddTwoScript(t, adapter)

		// use key that does not exist on service account
		invalidKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256,
			[]byte("invalid key invalid key invalid key invalid key invalid key invalid key"))
		invalidSigner, err := crypto.NewNaiveSigner(invalidKey, crypto.SHA3_256)
		require.NoError(t, err)

		tx := flowsdk.NewTransaction().
			SetScript([]byte(addTwoScript)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, invalidSigner)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.True(t, fvmerrors.HasErrorCode(result.Error, fvmerrors.ErrCodeInvalidProposalSignatureError))
	})

	t.Run("Key weights", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)

		accountKeys := test.AccountKeyGenerator()

		accountKeyA, signerA := accountKeys.NewWithSigner()
		accountKeyA.SetWeight(flowsdk.AccountKeyWeightThreshold / 2)

		accountKeyB, signerB := accountKeys.NewWithSigner()
		accountKeyB.SetWeight(flowsdk.AccountKeyWeightThreshold / 2)

		accountAddressA, err := adapter.CreateAccount(context.Background(), []*flowsdk.AccountKey{accountKeyA, accountKeyB}, nil)
		assert.NoError(t, err)

		script := []byte(`
		  transaction {
		    prepare(signer: &Account) {}
		  }
		`)

		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(accountAddressA, 1, 0).
			SetPayer(accountAddressA).
			AddAuthorizer(accountAddressA)

		// Insufficient keys
		err = tx.SignEnvelope(accountAddressA, 1, signerB)
		assert.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// Add key so we have sufficient keys
		err = tx.SignEnvelope(accountAddressA, 0, signerA)
		assert.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		t.Run("Insufficient key weight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			assert.True(t, fvmerrors.HasErrorCode(result.Error, fvmerrors.ErrCodeAccountAuthorizationError))
		})

		t.Run("Sufficient key weight", func(t *testing.T) {
			result, err := b.ExecuteNextTransaction()
			assert.NoError(t, err)

			AssertTransactionSucceeded(t, result)
		})
	})
}

func TestSubmitTransaction_PayloadSignatures(t *testing.T) {

	t.Parallel()

	t.Run("Multiple payload signers", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		accountKeys := test.AccountKeyGenerator()

		accountKeyB, signerB := accountKeys.NewWithSigner()
		accountKeyB.SetWeight(flowsdk.AccountKeyWeightThreshold)

		accountAddressB, err := adapter.CreateAccount(context.Background(), []*flowsdk.AccountKey{accountKeyB}, nil)
		assert.NoError(t, err)

		multipleAccountScript := []byte(`
		  transaction {
		    prepare(signerA: &Account, signerB: &Account) {
		      log(signerA.address)
			  log(signerB.address)
		    }
		  }
		`)

		tx := flowsdk.NewTransaction().
			SetScript(multipleAccountScript).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress).
			AddAuthorizer(accountAddressB)

		err = tx.SignPayload(accountAddressB, 0, signerB)
		assert.NoError(t, err)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		assert.Contains(t,
			result.Logs,
			interpreter.NewUnmeteredAddressValueFromBytes(serviceAccountAddress.Bytes()).String(),
		)

		assert.Contains(t,
			result.Logs,
			interpreter.NewUnmeteredAddressValueFromBytes(accountAddressB.Bytes()).String(),
		)
	})
}

func TestSubmitTransaction_Arguments(t *testing.T) {

	t.Parallel()

	addresses := test.AddressGenerator()

	fix64Value, _ := cadence.NewFix64("123456.00000")
	uFix64Value, _ := cadence.NewUFix64("123456.00000")

	var tests = []struct {
		argType cadence.Type
		arg     cadence.Value
	}{
		{
			cadence.BoolType,
			cadence.NewBool(true),
		},
		{
			cadence.StringType,
			cadence.String("foo"),
		},
		{
			cadence.AddressType,
			cadence.NewAddress(addresses.New()),
		},
		{
			cadence.IntType,
			cadence.NewInt(42),
		},
		{
			cadence.Int8Type,
			cadence.NewInt8(42),
		},
		{
			cadence.Int16Type,
			cadence.NewInt16(42),
		},
		{
			cadence.Int32Type,
			cadence.NewInt32(42),
		},
		{
			cadence.Int64Type,
			cadence.NewInt64(42),
		},
		{
			cadence.Int128Type,
			cadence.NewInt128(42),
		},
		{
			cadence.Int256Type,
			cadence.NewInt256(42),
		},
		{
			cadence.UIntType,
			cadence.NewUInt(42),
		},
		{
			cadence.UInt8Type,
			cadence.NewUInt8(42),
		},
		{
			cadence.UInt16Type,
			cadence.NewUInt16(42),
		},
		{
			cadence.UInt32Type,
			cadence.NewUInt32(42),
		},
		{
			cadence.UInt64Type,
			cadence.NewUInt64(42),
		},
		{
			cadence.UInt128Type,
			cadence.NewUInt128(42),
		},
		{
			cadence.UInt256Type,
			cadence.NewUInt256(42),
		},
		{
			cadence.Word8Type,
			cadence.NewWord8(42),
		},
		{
			cadence.Word16Type,
			cadence.NewWord16(42),
		},
		{
			cadence.Word32Type,
			cadence.NewWord32(42),
		},
		{
			cadence.Word64Type,
			cadence.NewWord64(42),
		},
		{
			cadence.Fix64Type,
			fix64Value,
		},
		{
			cadence.UFix64Type,
			uFix64Value,
		},
		{
			&cadence.ConstantSizedArrayType{
				Size:        3,
				ElementType: cadence.IntType,
			},
			cadence.NewArray([]cadence.Value{
				cadence.NewInt(1),
				cadence.NewInt(2),
				cadence.NewInt(3),
			}),
		},
		{
			&cadence.DictionaryType{
				KeyType:     cadence.StringType,
				ElementType: cadence.IntType,
			},
			cadence.NewDictionary([]cadence.KeyValuePair{
				{
					Key:   cadence.String("a"),
					Value: cadence.NewInt(1),
				},
				{
					Key:   cadence.String("b"),
					Value: cadence.NewInt(2),
				},
				{
					Key:   cadence.String("c"),
					Value: cadence.NewInt(3),
				},
			}),
		},
	}

	var script = func(argType cadence.Type) []byte {
		return []byte(fmt.Sprintf(`
            transaction(x: %s) {
              execute {
                log(x)
              }
            }
		`, argType.ID()))
	}

	for _, tt := range tests {
		t.Run(tt.argType.ID(), func(t *testing.T) {

			b, adapter := setupTransactionTests(t)
			serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

			tx := flowsdk.NewTransaction().
				SetScript(script(tt.argType)).
				SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
				SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
				SetPayer(serviceAccountAddress)

			err := tx.AddArgument(tt.arg)
			assert.NoError(t, err)

			signer, err := b.ServiceKey().Signer()
			require.NoError(t, err)

			err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
			require.NoError(t, err)

			err = adapter.SendTransaction(context.Background(), *tx)
			assert.NoError(t, err)

			result, err := b.ExecuteNextTransaction()
			require.NoError(t, err)
			AssertTransactionSucceeded(t, result)

			assert.Len(t, result.Logs, 1)
		})
	}

	t.Run("Log", func(t *testing.T) {
		b, adapter := setupTransactionTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		script := []byte(`
          transaction(x: Int) {
            execute {
              log(x * 6)
            }
          }
		`)

		x := 7

		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		err := tx.AddArgument(cadence.NewInt(x))
		assert.NoError(t, err)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		require.Len(t, result.Logs, 1)
		assert.Equal(t, "42", result.Logs[0])
	})
}

func TestSubmitTransaction_ProposerSequence(t *testing.T) {

	t.Parallel()

	t.Run("Valid transaction increases sequence number", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		script := []byte(`
		  transaction {
		    prepare(signer: &Account) {}
		  }
		`)
		prevSeq := b.ServiceKey().SequenceNumber

		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		tx1Result, err := adapter.GetTransactionResult(context.Background(), tx.ID())
		assert.NoError(t, err)
		assert.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)

		assert.Equal(t, prevSeq+1, b.ServiceKey().SequenceNumber)
	})

	t.Run("Reverted transaction increases sequence number", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		prevSeq := b.ServiceKey().SequenceNumber
		script := []byte(`
		  transaction {
			prepare(signer: &Account) {} 
			execute { panic("revert!") }
		  }
		`)

		tx := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		_, err = b.ExecuteNextTransaction()
		assert.NoError(t, err)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		tx1Result, err := adapter.GetTransactionResult(context.Background(), tx.ID())
		assert.NoError(t, err)
		assert.Equal(t, prevSeq+1, b.ServiceKey().SequenceNumber)
		assert.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)
		assert.Len(t, tx1Result.Events, 0)
		assert.IsType(t, &emulator.ExecutionError{}, tx1Result.Error)
	})
}

func TestGetTransaction(t *testing.T) {

	t.Parallel()

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
	)
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

	err = adapter.SendTransaction(context.Background(), *tx1)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	t.Run("Nonexistent", func(t *testing.T) {
		_, err := adapter.GetTransaction(context.Background(), flowsdk.EmptyID)
		if assert.Error(t, err) {
			assert.IsType(t, &emulator.TransactionNotFoundError{}, err)
		}
	})

	t.Run("Existent", func(t *testing.T) {
		tx2, err := adapter.GetTransaction(context.Background(), tx1.ID())
		require.NoError(t, err)

		assert.Equal(t, tx1.ID(), tx2.ID())
	})
}

func TestGetTransactionResult(t *testing.T) {

	t.Parallel()

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
	)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

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

	result, err := adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusUnknown, result.Status)
	require.Empty(t, result.Events)

	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	result, err = adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusPending, result.Status)
	require.Empty(t, result.Events)

	_, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)

	result, err = adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusPending, result.Status)
	require.Empty(t, result.Events)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	result, err = adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, result.Status)

	require.Len(t, result.Events, 3)

	event1 := result.Events[0]
	assert.Equal(t, tx.ID(), event1.TransactionID)
	assert.Equal(t, "flow.StorageCapabilityControllerIssued", event1.Type)
	assert.Equal(t, 0, event1.EventIndex)

	event2 := result.Events[1]
	assert.Equal(t, tx.ID(), event2.TransactionID)
	assert.Equal(t, "flow.CapabilityPublished", event2.Type)
	assert.Equal(t, 1, event2.EventIndex)

	event3 := result.Events[2]
	addr, _ := common.BytesToAddress(counterAddress.Bytes())
	location := common.AddressLocation{
		Address: addr,
		Name:    "Counting",
	}
	assert.Equal(t, tx.ID(), event3.TransactionID)
	assert.Equal(t,
		string(location.TypeID(nil, "Counting.CountIncremented")),
		event3.Type,
	)
	assert.Equal(t, 2, event3.EventIndex)
	fields := cadence.FieldsMappedByName(event3.Value)
	assert.Len(t, fields, 1)
	assert.Equal(t,
		cadence.NewInt(2),
		fields["count"],
	)
}

// TestGetTxByBlockIDMethods tests the GetTransactionByBlockID and GetTransactionResultByBlockID
// methods return the correct transaction and transaction result for a given block ID.
func TestGetTxByBlockIDMethods(t *testing.T) {

	t.Parallel()

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
	)

	const code = `
		transaction {
			execute {
				log("Hello, World!")
			}
		}
    `

	serviceKey := b.ServiceKey()
	serviceAccountAddress := flowsdk.Address(serviceKey.Address)

	signer, err := serviceKey.Signer()
	require.NoError(t, err)

	submittedTx := make([]*flowsdk.Transaction, 0)

	// submit 5 tx to be executed in a single block
	for i := uint64(0); i < 5; i++ {
		tx := flowsdk.NewTransaction().
			SetScript([]byte(code)).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, serviceKey.Index, serviceKey.SequenceNumber).
			SetPayer(serviceAccountAddress).
			AddAuthorizer(serviceAccountAddress)

		err = tx.SignEnvelope(serviceAccountAddress, serviceKey.Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// added to fix tx matching (nil vs empty slice)
		tx.PayloadSignatures = []flowsdk.TransactionSignature{}

		submittedTx = append(submittedTx, tx)

		// tx will be executed in the order they were submitted
		serviceKey.SequenceNumber++
	}

	// execute the batch of transactions
	block, expectedResults, err := b.ExecuteAndCommitBlock()
	assert.NoError(t, err)
	assert.Len(t, expectedResults, len(submittedTx))

	results, err := adapter.GetTransactionResultsByBlockID(context.Background(), flowsdk.Identifier(block.ID()))
	require.NoError(t, err)
	assert.Len(t, results, len(submittedTx))

	transactions, err := adapter.GetTransactionsByBlockID(context.Background(), flowsdk.Identifier(block.ID()))
	require.NoError(t, err)
	assert.Len(t, transactions, len(submittedTx))

	// make sure the results and transactions returned match the transactions submitted, and are in
	// the same order
	for i, tx := range submittedTx {
		assert.Equal(t, tx.ID(), transactions[i].ID())
		assert.Equal(t, submittedTx[i], transactions[i])

		assert.Equal(t, tx.ID(), results[i].TransactionID)
		assert.Equal(t, tx.ID(), expectedResults[i].TransactionID)
		// note: expectedResults from ExecuteAndCommitBlock and results from GetTransactionResultsByBlockID
		// use different representations. results is missing some data included in the flow.TransactionResult
		// struct, so we can't compare them directly.
	}
}

const helloWorldContract = `
    access(all) contract HelloWorld {

        access(all) fun hello(): String {
            return "Hello, World!"
        }
    }
`

const callHelloTxTemplate = `
    import HelloWorld from 0x%s
    transaction {
        prepare() {
            assert(HelloWorld.hello() == "Hello, World!")
        }
    }
`

func TestHelloWorld_NewAccount(t *testing.T) {

	t.Parallel()

	accountKeys := test.AccountKeyGenerator()

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
	)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)
	accountKey, accountSigner := accountKeys.NewWithSigner()

	contracts := []templates.Contract{
		{
			Name:   "HelloWorld",
			Source: helloWorldContract,
		},
	}

	createAccountTx, err := templates.CreateAccount(
		[]*flowsdk.AccountKey{accountKey},
		contracts,
		serviceAccountAddress,
	)
	require.NoError(t, err)

	createAccountTx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = createAccountTx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *createAccountTx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// createAccountTx status becomes TransactionStatusSealed
	createAccountTxResult, err := adapter.GetTransactionResult(context.Background(), createAccountTx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, createAccountTxResult.Status)

	var newAccountAddress flowsdk.Address
	for _, event := range createAccountTxResult.Events {
		if event.Type != flowsdk.EventAccountCreated {
			continue
		}
		accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
		newAccountAddress = accountCreatedEvent.Address()
		break
	}

	if newAccountAddress == flowsdk.EmptyAddress {
		assert.Fail(t, "missing account created event")
	}

	t.Logf("new account address: 0x%s", newAccountAddress.Hex())

	account, err := adapter.GetAccount(context.Background(), newAccountAddress)
	assert.NoError(t, err)

	assert.Equal(t, newAccountAddress, account.Address)

	// call hello world code

	accountKey = account.Keys[0]

	callHelloCode := []byte(fmt.Sprintf(callHelloTxTemplate, newAccountAddress.Hex()))
	callHelloTx := flowsdk.NewTransaction().
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetScript(callHelloCode).
		SetProposalKey(newAccountAddress, accountKey.Index, accountKey.SequenceNumber).
		SetPayer(newAccountAddress)

	err = callHelloTx.SignEnvelope(newAccountAddress, accountKey.Index, accountSigner)
	assert.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *callHelloTx)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestHelloWorld_UpdateAccount(t *testing.T) {

	t.Parallel()

	accountKeys := test.AccountKeyGenerator()

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
	)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	accountKey, accountSigner := accountKeys.NewWithSigner()
	_ = accountSigner

	contracts := []templates.Contract{
		{
			Name:   "HelloWorld",
			Source: `access(all) contract HelloWorld {}`,
		},
	}

	createAccountTx, err := templates.CreateAccount(
		[]*flowsdk.AccountKey{accountKey},
		contracts,
		serviceAccountAddress,
	)
	assert.NoError(t, err)

	createAccountTx.
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = createAccountTx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *createAccountTx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// createAccountTx status becomes TransactionStatusSealed
	createAccountTxResult, err := adapter.GetTransactionResult(context.Background(), createAccountTx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, createAccountTxResult.Status)

	var newAccountAddress flowsdk.Address
	for _, event := range createAccountTxResult.Events {
		if event.Type != flowsdk.EventAccountCreated {
			continue
		}
		accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
		newAccountAddress = accountCreatedEvent.Address()
		break
	}

	if newAccountAddress == flowsdk.EmptyAddress {
		assert.Fail(t, "missing account created event")
	}

	t.Logf("new account address: 0x%s", newAccountAddress.Hex())

	account, err := adapter.GetAccount(context.Background(), newAccountAddress)
	assert.NoError(t, err)

	accountKey = account.Keys[0]

	updateAccountCodeTx := templates.UpdateAccountContract(
		newAccountAddress,
		templates.Contract{
			Name:   "HelloWorld",
			Source: helloWorldContract,
		},
	)
	updateAccountCodeTx.
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(newAccountAddress, accountKey.Index, accountKey.SequenceNumber).
		SetPayer(newAccountAddress)

	err = updateAccountCodeTx.SignEnvelope(newAccountAddress, accountKey.Index, accountSigner)
	assert.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *updateAccountCodeTx)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// call hello world code

	accountKey.SequenceNumber++

	callHelloCode := []byte(fmt.Sprintf(callHelloTxTemplate, newAccountAddress.Hex()))
	callHelloTx := flowsdk.NewTransaction().
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetScript(callHelloCode).
		SetProposalKey(newAccountAddress, accountKey.Index, accountKey.SequenceNumber).
		SetPayer(newAccountAddress)

	err = callHelloTx.SignEnvelope(newAccountAddress, accountKey.Index, accountSigner)
	assert.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *callHelloTx)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestInfiniteTransaction(t *testing.T) {

	t.Parallel()

	const limit = 18

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
		emulator.WithTransactionMaxGasLimit(limit),
	)

	const code = `
		access(all) fun test() {
			test()
		}

		transaction {
			execute {
				test()
			}
		}
	`

	// Create a new account

	accountKeys := test.AccountKeyGenerator()
	accountKey, signer := accountKeys.NewWithSigner()
	accountAddress, err := adapter.CreateAccount(context.Background(), []*flowsdk.AccountKey{accountKey}, nil)
	assert.NoError(t, err)

	// Sign the transaction using the new account.
	// Do not test using the service account,
	// as the computation limit is disabled for it

	tx := flowsdk.NewTransaction().
		SetScript([]byte(code)).
		SetComputeLimit(limit).
		SetProposalKey(accountAddress, 0, 0).
		SetPayer(accountAddress)

	err = tx.SignEnvelope(accountAddress, 0, signer)
	assert.NoError(t, err)

	// Submit tx
	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	// Execute tx
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)

	require.True(t, fvmerrors.IsComputationLimitExceededError(result.Error))
}

func TestTransactionExecutionLimit(t *testing.T) {

	t.Parallel()

	const code = `
		transaction {
			execute {
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
		}
	`

	t.Run("ExceedingLimit", func(t *testing.T) {

		t.Parallel()

		const limit = 2000

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
			emulator.WithTransactionMaxGasLimit(limit),
		)

		// Create a new account

		accountKeys := test.AccountKeyGenerator()
		accountKey, signer := accountKeys.NewWithSigner()
		accountAddress, err := adapter.CreateAccount(context.Background(), []*flowsdk.AccountKey{accountKey}, nil)
		assert.NoError(t, err)

		// Sign the transaction using the new account.
		// Do not test using the service account,
		// as the computation limit is disabled for it

		tx := flowsdk.NewTransaction().
			SetScript([]byte(code)).
			SetComputeLimit(limit).
			SetProposalKey(accountAddress, 0, 0).
			SetPayer(accountAddress)

		err = tx.SignEnvelope(accountAddress, 0, signer)
		assert.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// Execute tx
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		require.True(t, fvmerrors.IsComputationLimitExceededError(result.Error))
	})

	t.Run("SufficientLimit", func(t *testing.T) {

		t.Parallel()

		const limit = 25000

		b, adapter := setupTransactionTests(
			t,
			emulator.WithStorageLimitEnabled(false),
			emulator.WithTransactionMaxGasLimit(limit),
		)

		// Create a new account

		accountKeys := test.AccountKeyGenerator()
		accountKey, signer := accountKeys.NewWithSigner()
		accountAddress, err := adapter.CreateAccount(context.Background(), []*flowsdk.AccountKey{accountKey}, nil)
		assert.NoError(t, err)

		// Sign the transaction using the new account.
		// Do not test using the service account,
		// as the computation limit is disabled for it

		tx := flowsdk.NewTransaction().
			SetScript([]byte(code)).
			SetComputeLimit(limit).
			SetProposalKey(accountAddress, 0, 0).
			SetPayer(accountAddress)

		err = tx.SignEnvelope(accountAddress, 0, signer)
		assert.NoError(t, err)

		// Submit tx
		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		// Execute tx
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.NoError(t, result.Error)
	})
}

func TestSubmitTransactionWithCustomLogger(t *testing.T) {

	t.Parallel()

	var memlog bytes.Buffer
	memlogWrite := io.Writer(&memlog)
	logger := zerolog.New(memlogWrite).Level(zerolog.DebugLevel)

	b, adapter := setupTransactionTests(
		t,
		emulator.WithStorageLimitEnabled(false),
		emulator.WithLogger(logger),
		emulator.WithTransactionFeesEnabled(true),
	)
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

	// Submit tx1
	err = adapter.SendTransaction(context.Background(), *tx1)
	assert.NoError(t, err)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionStatusSealed
	tx1Result, err := adapter.GetTransactionResult(context.Background(), tx1.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, tx1Result.Status)

	var meter Meter
	scanner := bufio.NewScanner(&memlog)
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Contains(txt, "transaction execution data") {
			err = json.Unmarshal([]byte(txt), &meter)
		}
	}

	assert.NoError(t, err)
	assert.Greater(t, meter.LedgerInteractionUsed, 0)
	assert.Greater(t, meter.ComputationUsed, 0)
	assert.Greater(t, meter.MemoryEstimate, 0)
	assert.Greater(t, len(meter.ComputationIntensities), 0)
	assert.Greater(t, len(meter.MemoryAmounts), 0)

}

type Meter struct {
	LedgerInteractionUsed  int                           `json:"ledgerInteractionUsed"`
	ComputationUsed        int                           `json:"computationUsed"`
	MemoryEstimate         int                           `json:"memoryEstimate"`
	ComputationIntensities MeteredComputationIntensities `json:"computationIntensities"`
	MemoryAmounts          MeteredMemoryAmounts          `json:"memoryAmounts"`
}

type MeteredComputationIntensities map[common.ComputationKind]uint64

type MeteredMemoryAmounts map[common.MemoryKind]uint64

func IncrementHelper(
	t *testing.T,
	b emulator.Emulator,
	adapter *emulator.SDKAdapter,
	counterAddress flowsdk.Address,
	addTwoScript string,
	expected int,
	expectSetup bool,
) {
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

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

	result, err := adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusUnknown, result.Status)
	require.Empty(t, result.Events)

	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	result, err = adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusPending, result.Status)
	require.Empty(t, result.Events)

	_, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)

	result, err = adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusPending, result.Status)
	require.Empty(t, result.Events)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	result, err = adapter.GetTransactionResult(context.Background(), tx.ID())
	assert.NoError(t, err)
	assert.Equal(t, flowsdk.TransactionStatusSealed, result.Status)

	var expectedEventIndex int
	if expectSetup {
		require.Len(t, result.Events, 3)

		event1 := result.Events[0]
		assert.Equal(t, tx.ID(), event1.TransactionID)
		assert.Equal(t, "flow.StorageCapabilityControllerIssued", event1.Type)
		assert.Equal(t, 0, event1.EventIndex)

		event2 := result.Events[1]
		assert.Equal(t, tx.ID(), event2.TransactionID)
		assert.Equal(t, "flow.CapabilityPublished", event2.Type)
		assert.Equal(t, 1, event2.EventIndex)

		expectedEventIndex = 2
	} else {
		require.Len(t, result.Events, 1)
		expectedEventIndex = 0
	}
	incrementedEvent := result.Events[expectedEventIndex]

	addr, _ := common.BytesToAddress(counterAddress.Bytes())
	location := common.AddressLocation{
		Address: addr,
		Name:    "Counting",
	}
	assert.Equal(t, tx.ID(), incrementedEvent.TransactionID)
	assert.Equal(t,
		string(location.TypeID(nil, "Counting.CountIncremented")),
		incrementedEvent.Type,
	)
	assert.Equal(t, expectedEventIndex, incrementedEvent.EventIndex)
	fields := cadence.FieldsMappedByName(incrementedEvent.Value)
	assert.Len(t, fields, 1)
	assert.Equal(t,
		cadence.NewInt(expected),
		fields["count"],
	)
}

// TestTransactionWithCadenceRandom checks Cadence's random function works
// within a transaction
func TestTransactionWithCadenceRandom(t *testing.T) {
	b, adapter := setupTransactionTests(t)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	code := `
    transaction {
        prepare() {
            assert(revertibleRandom<UInt64>() >= 0)
        }
    }
	`
	callRandomTx := flowsdk.NewTransaction().
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetScript([]byte(code)).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = callRandomTx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *callRandomTx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

func TestEVMTransaction(t *testing.T) {
	serviceAddr := flowgo.Emulator.Chain().ServiceAddress()
	code := []byte(fmt.Sprintf(
		`
		import EVM from %s

		transaction(bytes: [UInt8; 20]) {
			execute {
				let addr = EVM.EVMAddress(bytes: bytes)
				log(addr)
			}
		}
	 `,
		serviceAddr.HexWithPrefix(),
	))

	b, adapter := setupTransactionTests(t)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	// generate random address
	genArr := make([]cadence.Value, 20)
	for i := range genArr {
		genArr[i] = cadence.UInt8(i)
	}
	addressBytesArray := cadence.NewArray(genArr).WithType(stdlib.EVMAddressBytesCadenceType)

	tx := flowsdk.NewTransaction().
		SetScript(code).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	err := tx.AddArgument(addressBytesArray)
	assert.NoError(t, err)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	require.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	require.Len(t, result.Logs, 1)
	require.Equal(t, result.Logs[0], fmt.Sprintf("A.%s.EVM.EVMAddress(bytes: %s)", serviceAddr, addressBytesArray.String()))
}
