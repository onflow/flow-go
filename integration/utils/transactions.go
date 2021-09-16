package utils

import (
	"fmt"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
)

const (
	localnetNodePrivKey = "0xacb8175a98c8af74b248afc5e76fa207b3335103c435c6053d12a95da40262bc"

)


func EnvFromNetwork(network string) templates.Environment {
	if network == "mainnet" {
		return templates.Environment{
			// https://docs.onflow.org/protocol/core-contracts/flow-id-table-staking/
			IDTableAddress:       "8624b52f9ddcd04a",
			FungibleTokenAddress: "f233dcee88fe0abe",
			FlowTokenAddress:     "1654653399040a61",
			LockedTokensAddress:  "8d0e87b65159ae63",
			StakingProxyAddress:  "62430cf28c26d095",
		}
	}

	if network == "testnet" {
		return templates.Environment{
			IDTableAddress:       "9eca2b38b18b5dfe",
			FungibleTokenAddress: "9a0766d93b6608b7",
			FlowTokenAddress:     "7e60df042a9c0868",
			LockedTokensAddress:  "95e019a17d0e23d7",
			StakingProxyAddress:  "7aad92e5a0715d21",
		}
	}

	if network == "localnet" {
		return templates.Environment{
			IDTableAddress:       "f8d6e0586b0a20c7",
			FungibleTokenAddress: "ee82856bf20e2aa6",
			FlowTokenAddress:     "0ae53cb6e3f42a79",
			LockedTokensAddress:  "f8d6e0586b0a20c7",
			StakingProxyAddress:  "f8d6e0586b0a20c7",
		}
	}

	panic("invalid network string expecting one of ( mainnet | testnet | localnet )")
}

func MakeCreateLocalnetLeaseAccountWithKey(
	fullAccountKey *sdk.AccountKey,
	creatorAccount *sdk.Account,
	creatorAccountKeyIndex int,
	latestBlockID sdk.Identifier,
) *sdk.Transaction {
	creatorKey := creatorAccount.Keys[creatorAccountKeyIndex]

	adminPublicKey, err := crypto.DecodePublicKeyHex(
		crypto.ECDSA_P256,
		"b60899344f1779bb4c6df5a81db73e5781d895df06dee951e813eba99e76dd3c9288cceb5a5a9ada390671f60b71f3fd2653ca5c4e1ccc6f8b5a62be6d17256a",
	)
	if err != nil {
		panic(err)
	}
	fullAdminAccountKey := sdk.NewAccountKey().
		SetPublicKey(adminPublicKey).
		SetHashAlgo(crypto.SHA2_256).
		SetWeight(1000)

	tx := sdk.NewTransaction().
		SetScript([]byte(LockedTokenAccountCreationScriptLocalnet)).
		AddAuthorizer(creatorAccount.Address).
		AddRawArgument(jsoncdc.MustEncode(bytesToCadenceArray(fullAdminAccountKey.Encode()))).
		AddRawArgument(jsoncdc.MustEncode(bytesToCadenceArray(fullAccountKey.Encode()))).
		SetReferenceBlockID(latestBlockID).
		SetGasLimit(1000).
		SetProposalKey(creatorAccount.Address, creatorAccountKeyIndex, creatorKey.SequenceNumber).
		SetPayer(creatorAccount.Address)
	creatorKey.SequenceNumber++
	return tx
}

func MakeTransferLeaseToken(receiver sdk.Address, sender *sdk.Account, senderKeyID int, tokenAmount string, latestBlockID sdk.Identifier) (*sdk.Transaction, error) {
	senderKey := sender.Keys[senderKeyID]

	tx := sdk.NewTransaction().
		SetScript([]byte(LockedTransferScriptLocalnet)).
		SetGasLimit(1000).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(sender.Address, senderKeyID, senderKey.SequenceNumber).
		SetPayer(sender.Address).
		AddAuthorizer(sender.Address)

	err := tx.AddArgument(cadence.NewAddress(receiver))
	if err != nil {
		return nil, fmt.Errorf("could not add argument to transaction :%w", err)
	}

	amount, err := cadence.NewUFix64(tokenAmount)
	if err != nil {
		return nil, fmt.Errorf("could not add arguments to transaction :%w", err)
	}

	err = tx.AddArgument(amount)
	if err != nil {
		return nil, fmt.Errorf("could not add argument to transaction :%w", err)
	}

	senderKey.SequenceNumber++

	return tx, nil
}

// MakeCreateStakingCollectionTx submits transaction stakingCollection/setup_staking_collection.cdc
// which will setup the staking collection object for the account
func MakeCreateStakingCollectionTx(
	env templates.Environment,
	stakingAccount *sdk.Account,
	stakingAccountKeyID int,
	stakingSigner crypto.Signer,
	payerAddress sdk.Address,
	latestBlockID sdk.Identifier,
) (*sdk.Transaction, error) {
	accountKey := stakingAccount.Keys[stakingAccountKeyID]
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateCollectionSetup(env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(stakingAccount.Address, stakingAccountKeyID, accountKey.SequenceNumber).
		SetPayer(payerAddress).
		AddAuthorizer(stakingAccount.Address)

	//signing the payload as used AddAuthorizer
	err := tx.SignPayload(stakingAccount.Address, stakingAccountKeyID, stakingSigner)
	if err != nil {
		return nil, fmt.Errorf("could not sign payload: %w", err)
	}

	return tx, nil
}

func MakeStakeTx(
	env templates.Environment,
	stakingAccount *sdk.Account,
	stakingAccountKeyID int,
	stakingSigner crypto.Signer,
	payerAddress sdk.Address,
	latestBlockID sdk.Identifier,
) (*sdk.Transaction, error) {
	accountKey := stakingAccount.Keys[stakingAccountKeyID]
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateCollectionSetup(env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(stakingAccount.Address, stakingAccountKeyID, accountKey.SequenceNumber).
		SetPayer(payerAddress).
		AddAuthorizer(stakingAccount.Address)

	//signing the payload as used AddAuthorizer
	err := tx.SignPayload(stakingAccount.Address, stakingAccountKeyID, stakingSigner)
	if err != nil {
		return nil, fmt.Errorf("could not sign payload: %w", err)
	}

	return tx, nil
}

func LocalnetNodeAccount() (crypto.Signer, int, crypto.PublicKey, error) {
	sk, _, signer, err := FromKeyHex(crypto.SHA2_256, crypto.ECDSA_P256, localnetNodePrivKey)
	if err != nil {
		return nil, 0, nil, err
	}
	return signer, 0, sk.PublicKey(), nil
}

func FromKeyHex(hashAlgo crypto.HashAlgorithm, sigAlgo crypto.SignatureAlgorithm, keyHex string) (crypto.PrivateKey, *sdk.AccountKey, crypto.Signer, error) {
	sk, err := 	crypto.DecodePrivateKeyHex(sigAlgo, keyHex)
	if err != nil {
		return nil, nil, nil, err
	}
	accountKey := sdk.NewAccountKey().
		SetPublicKey(sk.PublicKey()).
		SetHashAlgo(hashAlgo).
		SetWeight(sdk.AccountKeyWeightThreshold)
	signer := crypto.NewInMemorySigner(sk, hashAlgo)
	return sk, accountKey, signer, nil
}
