package scripts

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/onflow/cadence"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed addKeysToAccountTransaction.cdc
var AddKeysToAccountTransaction []byte

//go:embed createAccountsTransaction.cdc
var createAccountsTransactionTemplate string

func CreateAccountsTransaction(fungibleToken, flowToken flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(createAccountsTransactionTemplate, fungibleToken, flowToken))
}

//go:embed compHeavyTransaction.cdc
var ComputationHeavyScriptTemplate string

//go:embed compHeavyContract.cdc
var ComputationHeavyContractTemplate string

//go:embed eventHeavyTransaction.cdc
var EventHeavyScriptTemplate string

//go:embed eventHeavyContract.cdc
var EventHeavyContractTemplate string

//go:embed ledgerHeavyTransaction.cdc
var LedgerHeavyScriptTemplate string

//go:embed ledgerHeavyContract.cdc
var LedgerHeavyContractTemplate string

//go:embed dataHeavyTransaction.cdc
var DataHeavyScriptTemplate string

//go:embed dataHeavyContract.cdc
var DataHeavyContractTemplate string

//go:embed tokenTransferTransaction.cdc
var tokenTransferTransactionTemplate string

//go:embed tokenTransferMultiTransaction.cdc
var tokenTransferMultiTransactionTemplate string

// TokenTransferTransaction returns a transaction script for transferring `amount` flow tokens to `toAddr` address
func TokenTransferTransaction(ftAddr, flowToken, toAddr flow.Address, amount cadence.UFix64) (*flowsdk.Transaction, error) {

	withFTAddr := strings.Replace(tokenTransferTransactionTemplate, "\"FungibleToken\"", "0x"+ftAddr.Hex(), 1)
	withFlowTokenAddr := strings.Replace(withFTAddr, "\"FlowToken\"", "0x"+flowToken.Hex(), 1)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(withFlowTokenAddr))

	err := tx.AddArgument(amount)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(cadence.BytesToAddress(toAddr.Bytes()))
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// TokenTransferMultiTransaction returns a transaction script for transferring `amount` flow tokens to all `toAddrs` addresses
func TokenTransferMultiTransaction(ftAddr, flowToken flow.Address, toAddrs []flow.Address, amount cadence.UFix64) (*flowsdk.Transaction, error) {
	withFTAddr := strings.Replace(tokenTransferMultiTransactionTemplate, "0xFUNGIBLETOKENADDRESS", "0x"+ftAddr.Hex(), 1)
	withFlowTokenAddr := strings.Replace(withFTAddr, "0xTOKENADDRESS", "0x"+flowToken.Hex(), 1)

	tx := flowsdk.NewTransaction().
		SetScript([]byte(withFlowTokenAddr))

	err := tx.AddArgument(amount)
	if err != nil {
		return nil, err
	}
	toAddrsArg := make([]cadence.Value, len(toAddrs))
	for i, addr := range toAddrs {
		toAddrsArg[i] = cadence.NewAddress(addr)
	}

	err = tx.AddArgument(cadence.NewArray(toAddrsArg))
	if err != nil {
		return nil, err
	}

	return tx, nil
}
