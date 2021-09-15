package utils

import (
	"fmt"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
)

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
	return tx
}

func MakeTransferLeaseToken(network string, receiver sdk.Address, sender *sdk.Account, senderKeyID int, tokenAmount string, latestBlock *sdk.Block) (sdk.Transaction, error) {
	senderKey := sender.Keys[senderKeyID]

	tx := sdk.NewTransaction().
		SetScript([]byte(LockedTransferScriptLocalnet)).
		SetGasLimit(1000).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(sender.Address, senderKeyID, senderKey.SequenceNumber).
		SetPayer(sender.Address).
		AddAuthorizer(sender.Address)

	err := tx.AddArgument(cadence.NewAddress(receiver))
	if err != nil {
		return sdk.Transaction{}, fmt.Errorf("could not add argument to transaction :%w", err)
	}

	amount, err := cadence.NewUFix64(tokenAmount)
	if err != nil {
		return sdk.Transaction{}, fmt.Errorf("could not add arguments to transaction :%w", err)
	}
	err = tx.AddArgument(amount)
	if err != nil {
		return sdk.Transaction{}, fmt.Errorf("could not add argument to transaction :%w", err)
	}

	senderKey.SequenceNumber++

	return *tx, nil
}
