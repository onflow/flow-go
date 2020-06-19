package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
)

const (
	FungibleTokenTransactionsBaseURL = "https://raw.githubusercontent.com/onflow/flow-ft/0e8024a483ce85c06eb165c2d4c9a5795ba167a1/src/transactions/"
	TransferTokens                   = "transfer_tokens.cdc"
)

type ScriptCreator struct {
	tokenTransferTemplate []byte
}

func NewScriptCreator() (*ScriptCreator, error) {
	ttt, err := getTokenTransferTemplate()
	if err != nil {
		return nil, err
	}
	return &ScriptCreator{tokenTransferTemplate: ttt}, nil
}

func (sc *ScriptCreator) TokenTransferScript(ftAddr, flowToken, toAddr *flowsdk.Address, amount int) ([]byte, error) {
	withFTAddr := strings.ReplaceAll(string(sc.tokenTransferTemplate), "0x02", "0x"+ftAddr.Hex())
	withFlowTokenAddr := strings.Replace(string(withFTAddr), "0x03", "0x"+flowToken.Hex(), 1)
	withToAddr := strings.Replace(string(withFlowTokenAddr), "0x04", "0x"+toAddr.Hex(), 1)
	withAmount := strings.Replace(string(withToAddr), fmt.Sprintf("%d.0", amount), "0.01", 1)
	return []byte(withAmount), nil
}

func (sc *ScriptCreator) CreateAccountScript(accountKey *flowsdk.AccountKey) ([]byte, error) {
	return templates.CreateAccount([]*flowsdk.AccountKey{accountKey}, nil)
}

func (sc *ScriptCreator) AddKeyToAccountScript(keys []*flowsdk.AccountKey) ([]byte, error) {
	publicKeysStr := strings.Builder{}
	for i := 0; i < len(keys); i++ {
		publicKeysStr.WriteString("signer.addPublicKey(")
		publicKeysStr.WriteString(languageEncodeBytes(keys[i].Encode()))
		publicKeysStr.WriteString(")\n")
	}
	script := fmt.Sprintf(`
	transaction {
	prepare(signer: AuthAccount) {
			%s
		}
	}`, publicKeysStr.String())

	return []byte(script), nil
}

func getTokenTransferTemplate() ([]byte, error) {
	resp, err := http.Get(FungibleTokenTransactionsBaseURL + TransferTokens)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// languageEncodeBytes converts a byte slice to a comma-separated list of uint8 integers.
func languageEncodeBytes(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}
