package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
)

const (
	fungibleTokenTransactionsBaseURL = "https://raw.githubusercontent.com/onflow/flow-ft/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/src/transactions/"
	transferTokens                   = "transfer_tokens.cdc"
)

// ScriptCreator creates transaction scripts
type ScriptCreator struct {
	tokenTransferTemplate []byte
}

// NewScriptCreator returns a new instance of ScriptCreator
func NewScriptCreator() (*ScriptCreator, error) {
	ttt, err := getTokenTransferTemplate()
	if err != nil {
		return nil, err
	}
	return &ScriptCreator{tokenTransferTemplate: ttt}, nil
}

// TokenTransferScript returns a transaction script for transfering `amount` flow tokens to `toAddr` address
func (sc *ScriptCreator) TokenTransferScript(ftAddr, flowToken, toAddr *flowsdk.Address, amount float64) ([]byte, error) {
	withFTAddr := strings.ReplaceAll(string(sc.tokenTransferTemplate), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.Replace(string(withFTAddr), "0x03", "0x"+flowToken.Hex(), 1)
	withToAddr := strings.Replace(string(withFlowTokenAddr), "0x04", "0x"+toAddr.Hex(), 1)
	withAmount := strings.Replace(string(withToAddr), fmt.Sprintf("%f", amount), "0.01", 1)
	return []byte(withAmount), nil
}

var addKeysScript = []byte(`
transaction(keys: [[UInt8]]) {
  prepare(signer: AuthAccount) {
	for key in keys {
	  signer.addPublicKey(key)
	}
  }
}
`)

// AddKeysToAccountTransaction returns a transaction for adding keys to an already existing account
func (sc *ScriptCreator) AddKeysToAccountTransaction(
	address flowsdk.Address,
	keys []*flowsdk.AccountKey,
) (*flowsdk.Transaction, error) {
	cadenceKeys := make([]cadence.Value, len(keys))

	for i, key := range keys {
		cadenceKeys[i] = bytesToCadenceArray(key.Encode())
	}

	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	tx := flowsdk.NewTransaction().
		SetScript(addKeysScript).
		AddAuthorizer(address)

	err := tx.AddArgument(cadenceKeysArray)
	if err != nil {
		return nil, err
	}

	return tx, err
}

func getTokenTransferTemplate() ([]byte, error) {
	resp, err := http.Get(fungibleTokenTransactionsBaseURL + transferTokens)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func bytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewUInt8(b)
	}

	return cadence.NewArray(values)
}
