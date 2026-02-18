package cohort3

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/antihax/optional"

	"github.com/onflow/cadence"
	ftcontracts "github.com/onflow/flow-ft/lib/go/contracts"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	nftcontracts "github.com/onflow/flow-nft/lib/go/contracts"
	swagger "github.com/onflow/flow/openapi/experimental/go-client-generated"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// setupExampleTokenVaultScript creates an ExampleToken vault on the signer's account.
	setupExampleTokenVaultScript = `
import FungibleToken from 0x%[1]s
import ExampleToken from 0x%[2]s

transaction {
    prepare(signer: auth(BorrowValue, SaveValue, Capabilities) &Account) {
        if signer.storage.borrow<&ExampleToken.Vault>(from: ExampleToken.VaultStoragePath) != nil {
            return
        }
        signer.storage.save(
            <-ExampleToken.createEmptyVault(vaultType: Type<@ExampleToken.Vault>()),
            to: ExampleToken.VaultStoragePath
        )
        signer.capabilities.publish(
            signer.capabilities.storage.issue<&{FungibleToken.Receiver}>(ExampleToken.VaultStoragePath),
            at: ExampleToken.ReceiverPublicPath
        )
        signer.capabilities.publish(
            signer.capabilities.storage.issue<&{FungibleToken.Balance}>(ExampleToken.VaultStoragePath),
            at: ExampleToken.VaultPublicPath
        )
    }
}
`

	// setupExampleNFTCollectionScript creates an ExampleNFT collection on the signer's account.
	setupExampleNFTCollectionScript = `
import NonFungibleToken from 0x%[1]s
import ExampleNFT from 0x%[2]s

transaction {
    prepare(signer: auth(BorrowValue, SaveValue, Capabilities) &Account) {
        if signer.storage.borrow<&ExampleNFT.Collection>(from: ExampleNFT.CollectionStoragePath) != nil {
            return
        }
        signer.storage.save(
            <-ExampleNFT.createEmptyCollection(nftType: Type<@ExampleNFT.NFT>()),
            to: ExampleNFT.CollectionStoragePath
        )
        signer.capabilities.publish(
            signer.capabilities.storage.issue<&{NonFungibleToken.Collection}>(ExampleNFT.CollectionStoragePath),
            at: ExampleNFT.CollectionPublicPath
        )
    }
}
`

	// sendExampleTokensScript transfers ExampleToken from the signer to the recipient.
	sendExampleTokensScript = `
import FungibleToken from 0x%[1]s
import ExampleToken from 0x%[2]s

transaction(amount: UFix64, to: Address) {
    let sentVault: @{FungibleToken.Vault}

    prepare(signer: auth(BorrowValue) &Account) {
        let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &ExampleToken.Vault>(from: ExampleToken.VaultStoragePath)
            ?? panic("Could not borrow reference to the owner's Vault!")
        self.sentVault <- vaultRef.withdraw(amount: amount)
    }

    execute {
        let receiverRef = getAccount(to)
            .capabilities.borrow<&{FungibleToken.Receiver}>(ExampleToken.ReceiverPublicPath)
            ?? panic("Could not borrow receiver reference to the recipient's Vault")
        receiverRef.deposit(from: <-self.sentVault)
    }
}
`

	// mintAndSendExampleNFTScript mints an ExampleNFT and deposits it to the recipient's collection.
	mintAndSendExampleNFTScript = `
import NonFungibleToken from 0x%[1]s
import ExampleNFT from 0x%[2]s
import MetadataViews from 0x%[3]s

transaction(recipient: Address) {
    let minter: &ExampleNFT.NFTMinter

    prepare(signer: auth(BorrowValue) &Account) {
        self.minter = signer.storage.borrow<&ExampleNFT.NFTMinter>(from: ExampleNFT.MinterStoragePath)
            ?? panic("Could not borrow a reference to the NFT minter")
    }

    execute {
        let receiverRef = getAccount(recipient)
            .capabilities.borrow<&{NonFungibleToken.Receiver}>(ExampleNFT.CollectionPublicPath)
            ?? panic("Could not get receiver reference to the recipient's NFT Collection")

        let nft <- self.minter.mintNFT(
            name: "Test NFT",
            description: "A test NFT for integration testing",
            thumbnail: "https://example.com/nft.png",
            royalties: []
        )
        receiverRef.deposit(token: <-nft)
    }
}
`
)

// TestAccountTransfers verifies the REST API endpoints for querying FT and NFT token transfers.
// It deploys custom FT (ExampleToken) and NFT (ExampleNFT) contracts, executes various transfers
// using both FlowToken and the custom tokens, and validates the transfer indexing via the REST API.
func (s *ExtendedIndexingSuite) TestAccountTransfers() {
	ctx := context.Background()

	serviceClient, err := s.net.ContainerByName(testnet.PrimaryAN).TestnetClient()
	s.Require().NoError(err)

	contracts := systemcontracts.SystemContractsForChain(flow.Localnet)
	serviceAddr := serviceClient.SDKServiceAddress()

	// Step 1: Deploy ExampleToken (custom FT) and ExampleNFT contracts on the service account.
	s.T().Log("deploying ExampleToken contract")
	exampleTokenCode := ftcontracts.ExampleToken(
		contracts.FungibleToken.Address.Hex(),
		contracts.MetadataViews.Address.Hex(),
		contracts.FungibleTokenMetadataViews.Address.Hex(),
	)
	s.deployContract(ctx, serviceClient, "ExampleToken", exampleTokenCode)

	s.T().Log("deploying ExampleNFT contract")
	exampleNFTCode := nftcontracts.ExampleNFT(
		sdk.Address(contracts.NonFungibleToken.Address),
		sdk.Address(contracts.MetadataViews.Address),
		sdk.Address(contracts.ViewResolver.Address),
	)
	s.deployContract(ctx, serviceClient, "ExampleNFT", exampleNFTCode)

	// Step 2: Create a new account.
	latestBlockID, err := serviceClient.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	accountPrivateKey := lib.RandomPrivateKey()
	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	newAccountAddress, _, err := serviceClient.CreateAccount(ctx, accountKey, sdk.Identifier(latestBlockID))
	s.Require().NoError(err)
	s.T().Logf("created new account: %s", newAccountAddress)

	newAddr := flow.Address(newAccountAddress)

	// Step 3: Create a client for the new account to sign setup transactions.
	grpcAddr := s.net.ContainerByName(testnet.PrimaryAN).Addr(testnet.GRPCPort)
	accountClient, err := testnet.NewClientWithKey(grpcAddr, newAccountAddress, accountPrivateKey, flow.Localnet.Chain())
	s.Require().NoError(err)

	// Step 4: Setup ExampleToken vault and ExampleNFT collection on the new account.
	s.T().Log("setting up ExampleToken vault on new account")
	s.sendTransaction(ctx, accountClient, fmt.Sprintf(
		setupExampleTokenVaultScript,
		contracts.FungibleToken.Address.Hex(),
		serviceAddr.Hex(),
	))

	s.T().Log("setting up ExampleNFT collection on new account")
	s.sendTransaction(ctx, accountClient, fmt.Sprintf(
		setupExampleNFTCollectionScript,
		contracts.NonFungibleToken.Address.Hex(),
		serviceAddr.Hex(),
	))

	// Step 5: Execute token transfers.
	// 5a. FlowToken transfer: service → new account
	s.T().Log("transferring FlowTokens to new account")
	flowTransferResult := s.sendTransferTx(ctx, serviceClient, newAccountAddress, "10.0", buildFlowTransferTx)

	// 5b. ExampleToken transfer: service → new account
	s.T().Log("transferring ExampleTokens to new account")
	exampleTokenTransferResult := s.sendExampleTokenTransferTx(ctx, serviceClient, newAccountAddress, "50.0", contracts)

	// 5c. ExampleNFT: mint on service and deposit to new account
	s.T().Log("minting ExampleNFT and sending to new account")
	nftMintResult := s.sendMintNFTTx(ctx, serviceClient, newAccountAddress, contracts)

	// Step 6: Wait for the extended indexer to catch up.
	maxHeight := nftMintResult.BlockHeight
	if flowTransferResult.BlockHeight > maxHeight {
		maxHeight = flowTransferResult.BlockHeight
	}
	if exampleTokenTransferResult.BlockHeight > maxHeight {
		maxHeight = exampleTokenTransferResult.BlockHeight
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 120*time.Second)
	defer waitCancel()
	err = serviceClient.WaitUntilIndexed(waitCtx, maxHeight+10)
	s.Require().NoError(err)
	s.T().Logf("indexer caught up to height %d", maxHeight+10)

	// Step 7: Verify FT transfers via the REST API.
	s.verifyFTTransfers(newAddr, flow.Address(serviceAddr))

	// Step 8: Verify NFT transfers via the REST API.
	s.verifyNFTTransfers(newAddr)
}

// deployContract deploys a contract with the given name and code on the service account.
func (s *ExtendedIndexingSuite) deployContract(
	ctx context.Context,
	client *testnet.Client,
	name string,
	code []byte,
) {
	latestBlockID, err := client.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	addr := client.Account().Address
	tx := templates.AddAccountContract(addr, templates.Contract{
		Name:   name,
		Source: string(code),
	})
	tx.SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(addr, 0, client.GetAndIncrementSeqNumber()).
		SetPayer(addr).
		SetComputeLimit(9999)

	err = client.SignAndSendTransaction(ctx, tx)
	s.Require().NoError(err)

	result, err := client.WaitForSealed(ctx, tx.ID())
	s.Require().NoError(err)
	s.Require().NoError(result.Error, "deploy %s failed", name)
	s.T().Logf("deployed %s at height %d", name, result.BlockHeight)
}

// sendTransaction builds, signs, sends, and waits for a transaction using the provided script.
// It uses the client's account address as the authorizer, proposer, and payer.
func (s *ExtendedIndexingSuite) sendTransaction(
	ctx context.Context,
	client *testnet.Client,
	script string,
	args ...cadence.Value,
) *sdk.TransactionResult {
	latestBlockID, err := client.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	addr := client.Account().Address

	tx := sdk.NewTransaction().
		SetScript([]byte(strings.TrimSpace(script))).
		AddAuthorizer(addr)

	for _, arg := range args {
		err = tx.AddArgument(arg)
		s.Require().NoError(err)
	}

	tx.SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(addr, 0, client.GetAndIncrementSeqNumber()).
		SetPayer(addr).
		SetComputeLimit(9999)

	err = client.SignAndSendTransaction(ctx, tx)
	s.Require().NoError(err)

	result, err := client.WaitForSealed(ctx, tx.ID())
	s.Require().NoError(err)
	s.Require().NoError(result.Error, "transaction failed: %s", result.Error)
	return result
}

// sendTransferTx builds and sends a FlowToken transfer transaction.
func (s *ExtendedIndexingSuite) sendTransferTx(
	ctx context.Context,
	client *testnet.Client,
	to sdk.Address,
	amount string,
	buildTx func(*testing.T, sdk.Address, string) *sdk.Transaction,
) *sdk.TransactionResult {
	latestBlockID, err := client.GetLatestBlockID(ctx)
	s.Require().NoError(err)

	tx := buildTx(s.T(), to, amount)
	tx.SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(client.SDKServiceAddress(), 0, client.GetAndIncrementSeqNumber()).
		SetPayer(client.SDKServiceAddress()).
		SetComputeLimit(9999)

	err = client.SignAndSendTransaction(ctx, tx)
	s.Require().NoError(err)

	result, err := client.WaitForSealed(ctx, tx.ID())
	s.Require().NoError(err)
	s.Require().NoError(result.Error, "FlowToken transfer failed")
	s.T().Logf("FlowToken transfer sealed at height %d, tx: %s", result.BlockHeight, tx.ID())
	return result
}

// sendExampleTokenTransferTx transfers ExampleToken from the service account to the recipient.
func (s *ExtendedIndexingSuite) sendExampleTokenTransferTx(
	ctx context.Context,
	client *testnet.Client,
	to sdk.Address,
	amount string,
	contracts *systemcontracts.SystemContracts,
) *sdk.TransactionResult {
	script := fmt.Sprintf(sendExampleTokensScript,
		contracts.FungibleToken.Address.Hex(),
		client.SDKServiceAddress().Hex(),
	)

	amountArg, err := cadence.NewUFix64(amount)
	s.Require().NoError(err)
	toArg := cadence.NewAddress(cadence.BytesToAddress(to.Bytes()))

	result := s.sendTransaction(ctx, client, script, amountArg, toArg)
	s.T().Logf("ExampleToken transfer sealed at height %d", result.BlockHeight)
	return result
}

// sendMintNFTTx mints an ExampleNFT and deposits it into the recipient's collection.
func (s *ExtendedIndexingSuite) sendMintNFTTx(
	ctx context.Context,
	client *testnet.Client,
	to sdk.Address,
	contracts *systemcontracts.SystemContracts,
) *sdk.TransactionResult {
	script := fmt.Sprintf(mintAndSendExampleNFTScript,
		contracts.NonFungibleToken.Address.Hex(),
		client.SDKServiceAddress().Hex(),
		contracts.MetadataViews.Address.Hex(),
	)

	toArg := cadence.NewAddress(cadence.BytesToAddress(to.Bytes()))
	result := s.sendTransaction(ctx, client, script, toArg)
	s.T().Logf("ExampleNFT mint sealed at height %d", result.BlockHeight)
	return result
}

// verifyFTTransfers validates FT transfer indexing via the REST API.
func (s *ExtendedIndexingSuite) verifyFTTransfers(
	recipientAddr flow.Address,
	senderAddr flow.Address,
) {
	ctx := context.Background()

	// Verify the recipient received both FlowToken and ExampleToken transfers.
	s.T().Log("verifying FT transfers for recipient")
	recipientTransfers := s.fetchFTTransfersEventually(recipientAddr.String(), nil, 2)
	s.T().Logf("recipient %s has %d FT transfers", recipientAddr, len(recipientTransfers))
	s.GreaterOrEqual(len(recipientTransfers), 2, "recipient should have at least 2 FT transfers (FlowToken + ExampleToken)")

	// Check that we see both token types.
	tokenTypes := make(map[string]bool)
	for _, transfer := range recipientTransfers {
		tokenTypes[transfer.TokenType] = true
		s.T().Logf("  FT transfer: token_type=%s amount=%s source=%s recipient=%s",
			transfer.TokenType, transfer.Amount, transfer.SourceAddress, transfer.RecipientAddress)
	}
	s.True(len(tokenTypes) >= 2, "should have transfers for at least 2 different token types, got: %v", tokenTypes)

	// Verify token_type filter returns only transfers of that type.
	for tokenType := range tokenTypes {
		filtered, err := s.apiClient.GetAllAccountFungibleTransfers(ctx, recipientAddr.String(), 50,
			&swagger.AccountsApiGetAccountFungibleTransfersOpts{
				TokenType: optional.NewString(tokenType),
			})
		s.Require().NoError(err)
		s.NotEmpty(filtered, "filtered results for token_type=%s should not be empty", tokenType)
		for _, t := range filtered {
			s.Equal(tokenType, t.TokenType, "filtered transfer should match requested token_type")
		}
	}

	// Verify role filter: recipient-only transfers for the recipient address.
	role := swagger.RECIPIENT_TransferRole
	recipientOnly, err := s.apiClient.GetAllAccountFungibleTransfers(ctx, recipientAddr.String(), 50,
		&swagger.AccountsApiGetAccountFungibleTransfersOpts{
			Role: optional.NewInterface(role),
		})
	s.Require().NoError(err)
	for _, t := range recipientOnly {
		s.Equal(recipientAddr.Hex(), t.RecipientAddress,
			"role=recipient filter should only return transfers where this account is the recipient")
	}

	// Verify the sender sent both FlowToken and ExampleToken.
	s.T().Log("verifying FT transfers for sender")
	senderTransfers := s.fetchFTTransfersEventually(senderAddr.String(), nil, 2)
	s.T().Logf("sender %s has %d FT transfers", senderAddr, len(senderTransfers))
	s.GreaterOrEqual(len(senderTransfers), 2, "sender should have at least 2 FT transfers")

	// Verify role filter: sender-only transfers for the sender address.
	senderRole := swagger.SENDER_TransferRole
	senderOnly, err := s.apiClient.GetAllAccountFungibleTransfers(ctx, senderAddr.String(), 50,
		&swagger.AccountsApiGetAccountFungibleTransfersOpts{
			Role: optional.NewInterface(senderRole),
		})
	s.Require().NoError(err)
	for _, t := range senderOnly {
		s.Equal(senderAddr.Hex(), t.SourceAddress,
			"role=sender filter should only return transfers where this account is the sender")
	}

	// Verify FT pagination: page through with limit=1, compare to unpaginated.
	s.T().Log("verifying FT pagination")
	allPaged, err := s.apiClient.GetAllAccountFungibleTransfers(ctx, recipientAddr.String(), 1, nil)
	s.Require().NoError(err)
	allUnpaged, err := s.apiClient.GetAllAccountFungibleTransfers(ctx, recipientAddr.String(), 50, nil)
	s.Require().NoError(err)
	s.Equal(len(allUnpaged), len(allPaged), "paginated and unpaginated FT transfer counts should match")
	for i := range allUnpaged {
		s.Equal(allUnpaged[i].TransactionId, allPaged[i].TransactionId,
			"FT transfer %d should have the same transaction ID in both paginated and unpaginated results", i)
	}
}

// verifyNFTTransfers validates NFT transfer indexing via the REST API.
func (s *ExtendedIndexingSuite) verifyNFTTransfers(
	recipientAddr flow.Address,
) {
	ctx := context.Background()

	// Verify the recipient received at least one NFT transfer.
	s.T().Log("verifying NFT transfers for recipient")
	recipientTransfers := s.fetchNFTTransfersEventually(recipientAddr.String(), nil, 1)
	s.T().Logf("recipient %s has %d NFT transfers", recipientAddr, len(recipientTransfers))
	s.GreaterOrEqual(len(recipientTransfers), 1, "recipient should have at least 1 NFT transfer")

	for _, transfer := range recipientTransfers {
		s.T().Logf("  NFT transfer: token_type=%s nft_id=%s source=%s recipient=%s",
			transfer.TokenType, transfer.NftId, transfer.SourceAddress, transfer.RecipientAddress)
		s.NotEmpty(transfer.TokenType, "NFT transfer should have a token_type")
		s.Equal(recipientAddr.Hex(), transfer.RecipientAddress, "recipient address should match")
	}

	// Verify role filter: recipient-only.
	role := swagger.RECIPIENT_TransferRole
	recipientOnly, err := s.apiClient.GetAllAccountNonFungibleTransfers(ctx, recipientAddr.String(), 50,
		&swagger.AccountsApiGetAccountNonFungibleTransfersOpts{
			Role: optional.NewInterface(role),
		})
	s.Require().NoError(err)
	for _, t := range recipientOnly {
		s.Equal(recipientAddr.Hex(), t.RecipientAddress,
			"role=recipient filter should only return transfers where this account is the recipient")
	}

	// Verify NFT pagination.
	s.T().Log("verifying NFT pagination")
	allPaged, err := s.apiClient.GetAllAccountNonFungibleTransfers(ctx, recipientAddr.String(), 1, nil)
	s.Require().NoError(err)
	allUnpaged, err := s.apiClient.GetAllAccountNonFungibleTransfers(ctx, recipientAddr.String(), 50, nil)
	s.Require().NoError(err)
	s.Equal(len(allUnpaged), len(allPaged), "paginated and unpaginated NFT transfer counts should match")
}

// fetchFTTransfersEventually retries fetching FT transfers until at least minCount results are returned.
func (s *ExtendedIndexingSuite) fetchFTTransfersEventually(
	address string,
	opts *swagger.AccountsApiGetAccountFungibleTransfersOpts,
	minCount int,
) []swagger.FungibleTokenTransfer {
	ctx := context.Background()

	var result []swagger.FungibleTokenTransfer
	s.Require().Eventually(func() bool {
		transfers, err := s.apiClient.GetAllAccountFungibleTransfers(ctx, address, 50, opts)
		if err != nil {
			s.T().Logf("FT transfers API request failed for %s: %v", address, err)
			return false
		}
		result = transfers
		return len(transfers) >= minCount
	}, 30*time.Second, 1*time.Second, "should get at least %d FT transfers for %s", minCount, address)

	return result
}

// fetchNFTTransfersEventually retries fetching NFT transfers until at least minCount results are returned.
func (s *ExtendedIndexingSuite) fetchNFTTransfersEventually(
	address string,
	opts *swagger.AccountsApiGetAccountNonFungibleTransfersOpts,
	minCount int,
) []swagger.NonFungibleTokenTransfer {
	ctx := context.Background()

	var result []swagger.NonFungibleTokenTransfer
	s.Require().Eventually(func() bool {
		transfers, err := s.apiClient.GetAllAccountNonFungibleTransfers(ctx, address, 50, opts)
		if err != nil {
			s.T().Logf("NFT transfers API request failed for %s: %v", address, err)
			return false
		}
		result = transfers
		return len(transfers) >= minCount
	}, 30*time.Second, 1*time.Second, "should get at least %d NFT transfers for %s", minCount, address)

	return result
}
