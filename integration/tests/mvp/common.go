package mvp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
)

// timeout for individual actions
const defaultTimeout = time.Second * 10

func RunMVPTest(t *testing.T, ctx context.Context, net *testnet.FlowNetwork, accessNode *testnet.Container) {

	chain := net.Root().Header.ChainID.Chain()

	serviceAccountClient, err := accessNode.TestnetClient()
	require.NoError(t, err)

	latestBlockID, err := serviceAccountClient.GetLatestBlockID(ctx)
	require.NoError(t, err)

	// create new account to deploy Counter to
	accountPrivateKey := lib.RandomPrivateKey()

	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	serviceAddress := sdk.Address(serviceAccountClient.Chain.ServiceAddress())

	// Generate the account creation transaction
	createAccountTx, err := templates.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		[]templates.Contract{
			{
				Name:   lib.CounterContract.Name,
				Source: lib.CounterContract.ToCadence(),
			},
		}, serviceAddress)
	require.NoError(t, err)
	createAccountTx.
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 0, serviceAccountClient.GetSeqNumber()).
		SetPayer(serviceAddress).
		SetComputeLimit(9999)

	childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	err = serviceAccountClient.SignAndSendTransaction(childCtx, createAccountTx)
	require.NoError(t, err)

	cancel()

	// wait for account to be created
	accountCreationTxRes, err := serviceAccountClient.WaitForSealed(context.Background(), createAccountTx.ID())
	require.NoError(t, err)
	t.Log(accountCreationTxRes)

	var newAccountAddress sdk.Address
	for _, event := range accountCreationTxRes.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			newAccountAddress = accountCreatedEvent.Address()
		}
	}
	require.NotEqual(t, sdk.EmptyAddress, newAccountAddress)

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	t.Log(">> new account address: ", newAccountAddress)

	// Generate the fund account transaction (so account can be used as a payer)
	fundAccountTx := sdk.NewTransaction().
		SetScript([]byte(fmt.Sprintf(`
			import FungibleToken from 0x%s
			import FlowToken from 0x%s

			transaction(amount: UFix64, recipient: Address) {
			  let sentVault: @FungibleToken.Vault
			  prepare(signer: AuthAccount) {
				let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
				  ?? panic("failed to borrow reference to sender vault")
				self.sentVault <- vaultRef.withdraw(amount: amount)
			  }
			  execute {
				let receiverRef =  getAccount(recipient)
				  .getCapability(/public/flowTokenReceiver)
				  .borrow<&{FungibleToken.Receiver}>()
					?? panic("failed to borrow reference to recipient vault")
				receiverRef.deposit(from: <-self.sentVault)
			  }
			}`,
			sc.FungibleToken.Address.Hex(),
			sc.FlowToken.Address.Hex(),
		))).
		AddAuthorizer(serviceAddress).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 0, serviceAccountClient.GetSeqNumber()).
		SetPayer(serviceAddress).
		SetComputeLimit(9999)

	err = fundAccountTx.AddArgument(cadence.UFix64(1_0000_0000))
	require.NoError(t, err)
	err = fundAccountTx.AddArgument(cadence.NewAddress(newAccountAddress))
	require.NoError(t, err)

	t.Log(">> funding new account...")

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = serviceAccountClient.SignAndSendTransaction(childCtx, fundAccountTx)
	require.NoError(t, err)

	cancel()

	fundCreationTxRes, err := serviceAccountClient.WaitForSealed(context.Background(), fundAccountTx.ID())
	require.NoError(t, err)
	t.Log(fundCreationTxRes)

	accountClient, err := testnet.NewClientWithKey(
		accessNode.Addr(testnet.GRPCPort),
		newAccountAddress,
		accountPrivateKey,
		chain,
	)
	require.NoError(t, err)

	// contract is deployed, but no instance is created yet
	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	counter, err := lib.ReadCounter(childCtx, accountClient, newAccountAddress)
	cancel()
	require.NoError(t, err)
	require.Equal(t, lib.CounterDefaultValue, counter)

	// create counter instance
	createCounterTx := sdk.NewTransaction().
		SetScript([]byte(lib.CreateCounterTx(newAccountAddress).ToCadence())).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(newAccountAddress, 0, 0).
		SetPayer(newAccountAddress).
		AddAuthorizer(newAccountAddress).
		SetComputeLimit(9999)

	t.Log(">> creating counter...")

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = accountClient.SignAndSendTransaction(ctx, createCounterTx)
	cancel()

	require.NoError(t, err)

	resp, err := accountClient.WaitForSealed(context.Background(), createCounterTx.ID())
	require.NoError(t, err)

	require.NoError(t, resp.Error)
	t.Log(resp)

	t.Log(">> awaiting counter incrementing...")

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = lib.ReadCounter(ctx, serviceAccountClient, newAccountAddress)
		cancel()

		return err == nil && counter == lib.CounterInitializedValue
	}, 30*time.Second, time.Second)
}
