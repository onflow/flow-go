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
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"

	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	emulator "github.com/onflow/flow-go/integration/internal/emulator"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const testContract = "access(all) contract Test {}"

func setupAccountTests(t *testing.T, opts ...emulator.Option) (
	*emulator.Blockchain,
	*emulator.SDKAdapter,
) {
	b, err := emulator.New(
		opts...,
	)
	require.NoError(t, err)

	logger := zerolog.Nop()
	return b, emulator.NewSDKAdapter(&logger, b)
}

func TestGetAccount(t *testing.T) {

	t.Parallel()

	t.Run("Get account at latest block height", func(t *testing.T) {

		t.Parallel()
		b, adapter := setupAccountTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)
		acc, err := adapter.GetAccount(context.Background(), serviceAccountAddress)
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), acc.Keys[0].SequenceNumber)

	})

	t.Run("Get account at latest block by index", func(t *testing.T) {

		t.Parallel()
		b, adapter := setupAccountTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		acc, err := adapter.GetAccount(context.Background(), serviceAccountAddress)
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), acc.Keys[0].SequenceNumber)

		flowAccount, err := b.GetAccountByIndex(1) //service account
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), flowAccount.Keys[0].SeqNumber)
		assert.Equal(t, acc.Address.String(), flowAccount.Address.String())

	})

	t.Run("Get account at latest block height", func(t *testing.T) {

		t.Parallel()
		b, adapter := setupAccountTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		acc, err := adapter.GetAccount(context.Background(), serviceAccountAddress)
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), acc.Keys[0].SequenceNumber)

		flowAccount, err := b.GetAccountByIndex(1) //service account
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), flowAccount.Keys[0].SeqNumber)
		assert.Equal(t, acc.Address.String(), flowAccount.Address.String())

	})

	t.Run("Get account at specified block height", func(t *testing.T) {

		t.Parallel()

		b, adapter := setupAccountTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		acc, err := adapter.GetAccount(context.Background(), serviceAccountAddress)
		assert.NoError(t, err)

		assert.Equal(t, uint64(0), acc.Keys[0].SequenceNumber)
		contract := templates.Contract{
			Name:   "Test",
			Source: testContract,
		}

		tx := templates.AddAccountContract(serviceAccountAddress, contract)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
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

		bl, err := b.CommitBlock()
		assert.NoError(t, err)

		accNow, err := adapter.GetAccountAtBlockHeight(context.Background(), serviceAccountAddress, bl.Height)
		assert.NoError(t, err)

		accPrev, err := adapter.GetAccountAtBlockHeight(context.Background(), serviceAccountAddress, bl.Height-uint64(1))
		assert.NoError(t, err)

		assert.Equal(t, accNow.Keys[0].SequenceNumber, uint64(1))
		assert.Equal(t, accPrev.Keys[0].SequenceNumber, uint64(0))
	})
}

func TestCreateAccount(t *testing.T) {

	t.Parallel()

	accountKeys := test.AccountKeyGenerator()

	t.Run("Simple addresses", func(t *testing.T) {
		b, adapter := setupAccountTests(
			t,
			emulator.WithSimpleAddresses(),
		)

		accountKey := accountKeys.New()
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKey},
			nil,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
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

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := LastCreatedAccount(b, result)
		require.NoError(t, err)

		assert.Equal(t, "0000000000000006", account.Address.Hex())
		assert.Equal(t, uint64(0x186a0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Empty(t, account.Contracts)
	})

	t.Run("Single public keys", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		accountKey := accountKeys.New()
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKey},
			nil,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
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

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := LastCreatedAccount(b, result)
		require.NoError(t, err)

		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Empty(t, account.Contracts)
	})

	t.Run("Multiple public keys", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		accountKeyA := accountKeys.New()
		accountKeyB := accountKeys.New()
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKeyA, accountKeyB},
			nil,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := LastCreatedAccount(b, result)
		require.NoError(t, err)

		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Equal(t, accountKeyB.PublicKey.Encode(), account.Keys[1].PublicKey.Encode())
		assert.Empty(t, account.Contracts)
	})

	t.Run("Public keys and contract", func(t *testing.T) {
		b, adapter := setupAccountTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		accountKeyA := accountKeys.New()
		accountKeyB := accountKeys.New()

		contracts := []templates.Contract{
			{
				Name:   "Test",
				Source: testContract,
			},
		}

		tx, err := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKeyA, accountKeyB},
			contracts,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := LastCreatedAccount(b, result)
		require.NoError(t, err)

		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Equal(t, accountKeyB.PublicKey.Encode(), account.Keys[1].PublicKey.Encode())
		assert.Equal(t,
			map[string][]byte{
				"Test": []byte(testContract),
			},
			account.Contracts,
		)
	})

	t.Run("Public keys and two contracts", func(t *testing.T) {
		b, adapter := setupAccountTests(t)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		codeA := `
		  access(all) contract Test1 {
			  access(all) fun a(): Int {
				  return 1
			  }
		  }
		`
		codeB := `
		  access(all) contract Test2 {
			  access(all) fun b(): Int {
				  return 2
			  }
		  }
		`

		accountKey := accountKeys.New()

		contracts := []templates.Contract{
			{
				Name:   "Test1",
				Source: codeA,
			},
			{
				Name:   "Test2",
				Source: codeB,
			},
		}

		tx, err := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKey},
			contracts,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		require.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := LastCreatedAccount(b, result)
		require.NoError(t, err)

		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey.Encode(), account.Keys[0].PublicKey.Encode())
		assert.Equal(t,
			map[string][]byte{
				"Test1": []byte(codeA),
				"Test2": []byte(codeB),
			},
			account.Contracts,
		)
	})

	t.Run("Code and no keys", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		contracts := []templates.Contract{
			{
				Name:   "Test",
				Source: testContract,
			},
		}
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			nil,
			contracts,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err := LastCreatedAccount(b, result)
		require.NoError(t, err)

		assert.Empty(t, account.Keys)
		assert.Equal(t,
			map[string][]byte{
				"Test": []byte(testContract),
			},
			account.Contracts,
		)
	})

	t.Run("Event emitted", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		accountKey := accountKeys.New()

		contracts := []templates.Contract{
			{
				Name:   "Test",
				Source: testContract,
			},
		}
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKey},
			contracts,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		block, err := b.CommitBlock()
		require.NoError(t, err)

		events, err := adapter.GetEventsForHeightRange(context.Background(), flowsdk.EventAccountCreated, block.Height, block.Height)
		require.NoError(t, err)
		require.Len(t, events, 1)

		accountEvent := flowsdk.AccountCreatedEvent(events[0].Events[0])

		account, err := adapter.GetAccount(context.Background(), accountEvent.Address())
		assert.NoError(t, err)

		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey.PublicKey, account.Keys[0].PublicKey)
		assert.Equal(t,
			map[string][]byte{
				"Test": []byte(testContract),
			},
			account.Contracts,
		)
	})

	t.Run("Invalid hash algorithm", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		accountKey := accountKeys.New()
		accountKey.SetHashAlgo(crypto.SHA3_384) // SHA3_384 is invalid for ECDSA_P256
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			[]*flowsdk.AccountKey{accountKey},
			nil,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)

		assert.True(t, result.Reverted())
	})

	t.Run("Invalid code", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		contracts := []templates.Contract{
			{
				Name:   "Test",
				Source: "not a valid script",
			},
		}
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			nil,
			contracts,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)

		assert.True(t, result.Reverted())
	})

	t.Run("Invalid contract name", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		contracts := []templates.Contract{
			{
				Name:   "Test2",
				Source: testContract,
			},
		}
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.CreateAccount(
			nil,
			contracts,
			serviceAccountAddress,
		)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		require.NoError(t, err)

		assert.True(t, result.Reverted())
	})
}

func TestAddAccountKey(t *testing.T) {

	t.Parallel()

	accountKeys := test.AccountKeyGenerator()

	t.Run("Valid key", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		newAccountKey, newSigner := accountKeys.NewWithSigner()
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx1, err := templates.AddAccountKey(serviceAccountAddress, newAccountKey)
		assert.NoError(t, err)

		tx1.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx1.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx1)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		script := []byte("transaction { execute {} }")

		var newKeyID = uint32(1) // new key will have ID 1
		var newKeySequenceNum uint64 = 0

		tx2 := flowsdk.NewTransaction().
			SetScript(script).
			SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, newKeyID, newKeySequenceNum).
			SetPayer(serviceAccountAddress)

		err = tx2.SignEnvelope(serviceAccountAddress, newKeyID, newSigner)
		assert.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx2)
		require.NoError(t, err)

		result, err = b.ExecuteNextTransaction()
		require.NoError(t, err)
		AssertTransactionSucceeded(t, result)

		_, err = b.CommitBlock()
		assert.NoError(t, err)
	})

	t.Run("Invalid hash algorithm", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		accountKey := accountKeys.New()
		accountKey.SetHashAlgo(crypto.SHA3_384) // SHA3_384 is invalid for ECDSA_P256
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx, err := templates.AddAccountKey(serviceAccountAddress, accountKey)
		assert.NoError(t, err)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())
	})
}

func TestRemoveAccountKey(t *testing.T) {

	t.Parallel()

	b, adapter := setupAccountTests(t)

	accountKeys := test.AccountKeyGenerator()

	newAccountKey, newSigner := accountKeys.NewWithSigner()
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	// create transaction that adds public key to account keys
	tx1, err := templates.AddAccountKey(serviceAccountAddress, newAccountKey)
	assert.NoError(t, err)

	// create transaction that adds public key to account keys
	tx1.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	// sign with service key
	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx1.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	// submit tx1 (should succeed)
	err = adapter.SendTransaction(context.Background(), *tx1)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err := adapter.GetAccount(context.Background(), serviceAccountAddress)
	assert.NoError(t, err)

	require.Len(t, account.Keys, 2)
	assert.False(t, account.Keys[0].Revoked)
	assert.False(t, account.Keys[1].Revoked)

	// create transaction that removes service key
	tx2 := templates.RemoveAccountKey(serviceAccountAddress, 0)

	tx2.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	// sign with service key
	signer, err = b.ServiceKey().Signer()
	assert.NoError(t, err)
	err = tx2.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	assert.NoError(t, err)

	// submit tx2 (should succeed)
	err = adapter.SendTransaction(context.Background(), *tx2)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = adapter.GetAccount(context.Background(), serviceAccountAddress)
	assert.NoError(t, err)

	// key at index 0 should be revoked
	require.Len(t, account.Keys, 2)
	assert.True(t, account.Keys[0].Revoked)
	assert.False(t, account.Keys[1].Revoked)

	// create transaction that removes remaining account key
	tx3 := templates.RemoveAccountKey(serviceAccountAddress, 0)

	tx3.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	// sign with service key (that has been removed)
	signer, err = b.ServiceKey().Signer()
	assert.NoError(t, err)
	err = tx3.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	assert.NoError(t, err)

	// submit tx3 (should fail)
	err = adapter.SendTransaction(context.Background(), *tx3)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)

	var sigErr fvmerrors.CodedError
	assert.ErrorAs(t, result.Error, &sigErr)
	assert.True(t, fvmerrors.HasErrorCode(result.Error, fvmerrors.ErrCodeInvalidProposalSignatureError))

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = adapter.GetAccount(context.Background(), serviceAccountAddress)
	assert.NoError(t, err)

	// key at index 1 should not be revoked
	require.Len(t, account.Keys, 2)
	assert.True(t, account.Keys[0].Revoked)
	assert.False(t, account.Keys[1].Revoked)

	// create transaction that removes remaining account key
	tx4 := templates.RemoveAccountKey(serviceAccountAddress, 1)

	tx4.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, account.Keys[1].Index, account.Keys[1].SequenceNumber).
		SetPayer(serviceAccountAddress)

	// sign with remaining account key
	err = tx4.SignEnvelope(serviceAccountAddress, account.Keys[1].Index, newSigner)
	assert.NoError(t, err)

	// submit tx4 (should succeed)
	err = adapter.SendTransaction(context.Background(), *tx4)
	assert.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	account, err = adapter.GetAccount(context.Background(), serviceAccountAddress)
	assert.NoError(t, err)

	// all keys should be revoked
	for _, key := range account.Keys {
		assert.True(t, key.Revoked)
	}
}

func TestUpdateAccountCode(t *testing.T) {

	t.Parallel()

	const codeA = `
      access(all) contract Test {
          access(all) fun a(): Int {
              return 1
          }
      }
    `

	const codeB = `
      access(all) contract Test {
          access(all) fun b(): Int {
              return 2
          }
      }
    `

	accountKeys := test.AccountKeyGenerator()

	accountKeyB, signerB := accountKeys.NewWithSigner()

	t.Run("Valid signature", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		contracts := []templates.Contract{
			{
				Name:   "Test",
				Source: codeA,
			},
		}

		accountAddressB, err := adapter.CreateAccount(
			context.Background(),
			[]*flowsdk.AccountKey{accountKeyB},
			contracts,
		)
		require.NoError(t, err)

		account, err := adapter.GetAccount(context.Background(), accountAddressB)
		require.NoError(t, err)

		assert.Equal(t,
			map[string][]byte{
				"Test": []byte(codeA),
			},
			account.Contracts,
		)

		tx := templates.UpdateAccountContract(
			accountAddressB,
			templates.Contract{
				Name:   "Test",
				Source: codeB,
			},
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

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

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err = adapter.GetAccount(context.Background(), accountAddressB)
		assert.NoError(t, err)

		assert.Equal(t, codeB, string(account.Contracts["Test"]))
	})

	t.Run("Invalid signature", func(t *testing.T) {
		b, adapter := setupAccountTests(t)

		contracts := []templates.Contract{
			{
				Name:   "Test",
				Source: codeA,
			},
		}

		accountAddressB, err := adapter.CreateAccount(
			context.Background(),
			[]*flowsdk.AccountKey{accountKeyB},
			contracts,
		)
		require.NoError(t, err)

		account, err := adapter.GetAccount(context.Background(), accountAddressB)
		require.NoError(t, err)

		assert.Equal(t, codeA, string(account.Contracts["Test"]))

		tx := templates.UpdateAccountContract(
			accountAddressB,
			templates.Contract{
				Name:   "Test",
				Source: codeB,
			},
		)
		serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

		tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
			SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
			SetPayer(serviceAccountAddress)

		// invalid authorizer signature
		tx.AddPayloadSignature(accountAddressB, 0, unittest.SignatureFixtureForTransactions())

		signer, err := b.ServiceKey().Signer()
		require.NoError(t, err)

		err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
		require.NoError(t, err)

		err = adapter.SendTransaction(context.Background(), *tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)

		assert.True(t, fvmerrors.HasErrorCode(result.Error, fvmerrors.ErrCodeInvalidProposalSignatureError))

		_, err = b.CommitBlock()
		assert.NoError(t, err)

		account, err = adapter.GetAccount(context.Background(), accountAddressB)
		assert.NoError(t, err)

		// code should not be updated
		assert.Equal(t, codeA, string(account.Contracts["Test"]))
	})
}

func TestImportAccountCode(t *testing.T) {

	t.Parallel()

	b, adapter := setupAccountTests(t)

	accountContracts := []templates.Contract{
		{
			Name: "Computer",
			Source: `
              access(all) contract Computer {
                  access(all) fun answer(): Int {
                      return 42
                  }
              }
	        `,
		},
	}

	address, err := adapter.CreateAccount(context.Background(), nil, accountContracts)
	assert.NoError(t, err)

	script := []byte(fmt.Sprintf(`
		// address imports can omit leading zeros
		import 0x%s

		transaction {
		  execute {
			let answer = Computer.answer()
			if answer != 42 {
				panic("?!")
			}
		  }
		}
	`, address))
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	tx := flowsdk.NewTransaction().
		SetScript(script).
		SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	signer, err := b.ServiceKey().Signer()
	require.NoError(t, err)

	err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *tx)
	assert.NoError(t, err)

	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	AssertTransactionSucceeded(t, result)
}

func TestAccountAccess(t *testing.T) {

	t.Parallel()

	b, adapter := setupAccountTests(t)

	// Create first account and deploy a contract A
	// which has a field
	// which only other code in the same should be allowed to access

	accountContracts := []templates.Contract{
		{
			Name: "A",
			Source: `
				access(all) contract A {
					access(account) let a: Int

					init() {
						self.a = 1
					}
				}
			`,
		},
	}

	accountKeys := test.AccountKeyGenerator()

	accountKey1, signer1 := accountKeys.NewWithSigner()

	address1, err := adapter.CreateAccount(
		context.Background(),
		[]*flowsdk.AccountKey{accountKey1},
		accountContracts,
	)
	assert.NoError(t, err)

	// Deploy another contract B to the same account
	// which accesses the field in contract A
	// which allows access to code in the same account

	tx := templates.AddAccountContract(
		address1,
		templates.Contract{
			Name: "B",
			Source: fmt.Sprintf(`
				    import A from 0x%s

					access(all) contract B {
						access(all) fun use() {
							let b = A.a
						}
					}
				`,
				address1.Hex(),
			),
		},
	)
	serviceAccountAddress := flowsdk.Address(b.ServiceKey().Address)

	tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	err = tx.SignPayload(address1, 0, signer1)
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

	_, err = b.CommitBlock()
	require.NoError(t, err)

	// Create another account 2

	accountKey2, signer2 := accountKeys.NewWithSigner()

	address2, err := adapter.CreateAccount(
		context.Background(),
		[]*flowsdk.AccountKey{accountKey2},
		nil,
	)
	assert.NoError(t, err)

	// Deploy a contract C to the second account
	// which accesses the field in contract A of the first account
	// which allows access to code in the same account

	tx = templates.AddAccountContract(
		address2,
		templates.Contract{
			Name: "C",
			Source: fmt.Sprintf(`
				    import A from 0x%s

					access(all) contract C {
						access(all) fun use() {
							let b = A.a
						}
					}
				`,
				address1.Hex(),
			),
		},
	)

	tx.SetComputeLimit(flowgo.DefaultMaxTransactionGasLimit).
		SetProposalKey(serviceAccountAddress, b.ServiceKey().Index, b.ServiceKey().SequenceNumber).
		SetPayer(serviceAccountAddress)

	err = tx.SignPayload(address2, 0, signer2)
	require.NoError(t, err)

	signer, err = b.ServiceKey().Signer()
	assert.NoError(t, err)
	err = tx.SignEnvelope(serviceAccountAddress, b.ServiceKey().Index, signer)
	require.NoError(t, err)

	err = adapter.SendTransaction(context.Background(), *tx)
	require.NoError(t, err)

	result, err = b.ExecuteNextTransaction()
	require.NoError(t, err)

	require.False(t, result.Succeeded())
	require.Error(t, result.Error)

	require.Contains(
		t,
		result.Error.Error(),
		"error: access denied: cannot access `a` because field requires `account` authorization",
	)
}
