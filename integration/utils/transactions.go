package utils

import (
	"fmt"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	fttemplates "github.com/onflow/flow-ft/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/model/flow"
)

func LocalnetEnv() templates.Environment {
	return templates.Environment{
		IDTableAddress:       "f8d6e0586b0a20c7",
		FungibleTokenAddress: "ee82856bf20e2aa6",
		FlowTokenAddress:     "0ae53cb6e3f42a79",
		LockedTokensAddress:  "f8d6e0586b0a20c7",
		StakingProxyAddress:  "f8d6e0586b0a20c7",
	}
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
	accountKey.SequenceNumber++
	return tx, nil
}

func MakeCollectionRegisterNodeTx(
	env templates.Environment,
	stakingAccount *sdk.Account,
	stakingAccountKeyID int,
	stakingSigner crypto.Signer,
	payerAddress sdk.Address,
	latestBlockID sdk.Identifier,
	nodeID flow.Identifier,
	role flow.Role,
	networkingAddress string,
	networkingKey string,
	stakingKey string,
	amount string,
	machineKey string,
) (*sdk.Transaction, error) {
	accountKey := stakingAccount.Keys[stakingAccountKeyID]
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateCollectionRegisterNode(env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(stakingAccount.Address, stakingAccountKeyID, accountKey.SequenceNumber).
		SetPayer(payerAddress).
		AddAuthorizer(stakingAccount.Address)

	id, _ := cadence.NewString(nodeID.String())
	err := tx.AddArgument(id)
	if err != nil {
		return nil, err
	}

	r := cadence.NewUInt8(uint8(role))
	err = tx.AddArgument(r)
	if err != nil {
		return nil, err
	}

	networkingAddressCDC, _ := cadence.NewString(networkingAddress)
	err = tx.AddArgument(networkingAddressCDC)
	if err != nil {
		return nil, err
	}

	networkingKeyCDC, _ := cadence.NewString(networkingKey)
	err = tx.AddArgument(networkingKeyCDC)
	if err != nil {
		return nil, err
	}

	stakingKeyCDC, _ := cadence.NewString(stakingKey)
	err = tx.AddArgument(stakingKeyCDC)
	if err != nil {
		return nil, err
	}

	amountCDC, _ := cadence.NewUFix64(amount)
	err = tx.AddArgument(amountCDC)
	if err != nil {
		return nil, err
	}

	machineKeyCDC, _ := cadence.NewString(machineKey)
	publicKeys := make([]cadence.Value, 1)
	publicKeys[0] = machineKeyCDC
	publicKeysCDC := cadence.NewArray(publicKeys)

	err = tx.AddArgument(cadence.NewOptional(publicKeysCDC))
	if err != nil {
		return nil, err
	}

	err = tx.SignPayload(stakingAccount.Address, stakingAccountKeyID, stakingSigner)
	if err != nil {
		return nil, fmt.Errorf("could not sign payload: %w", err)
	}

	return tx, nil
}

func MakeTransferTokenTx(env templates.Environment, receiver sdk.Address, sender *sdk.Account, senderKeyID int, tokenAmount string, latestBlockID sdk.Identifier) (
	*sdk.Transaction, error) {

	senderKey := sender.Keys[senderKeyID]
	fungible := sdk.HexToAddress(env.FungibleTokenAddress)
	flowToken := sdk.HexToAddress(env.FlowTokenAddress)
	script := fttemplates.GenerateTransferVaultScript(fungible, flowToken, "FlowToken")

	tx := sdk.NewTransaction().
		SetScript([]byte(script)).
		SetGasLimit(100).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(sender.Address, senderKeyID, senderKey.SequenceNumber).
		SetPayer(sender.Address).
		AddAuthorizer(sender.Address)

	amount, err := cadence.NewUFix64(tokenAmount)
	if err != nil {
		return nil, fmt.Errorf("could not add arguments to transaction :%w", err)
	}
	err = tx.AddArgument(amount)
	if err != nil {
		return nil, fmt.Errorf("could not add argument to transaction :%w", err)
	}

	err = tx.AddArgument(cadence.NewAddress(receiver))
	if err != nil {
		return nil, fmt.Errorf("could not add argument to transaction :%w", err)
	}
	senderKey.SequenceNumber++
	return tx, nil
}
