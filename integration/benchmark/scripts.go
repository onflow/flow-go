package benchmark

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"

	"github.com/onflow/cadence"

	flowsdk "github.com/onflow/flow-go-sdk"
)

//go:embed scripts/createAccountsTransaction.cdc
var createAccountsScriptTemplate string

// CreateAccountsScript returns a transaction script for creating an account
func CreateAccountsScript(fungibleToken, flowToken flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(createAccountsScriptTemplate, fungibleToken, flowToken))
}

//go:embed scripts/myFavContract.cdc
var myFavContract string

//go:embed scripts/deployingMyFavContractTransaction.cdc
var deployingMyFavContractScriptTemplate string

func DeployingMyFavContractScript() []byte {
	return []byte(fmt.Sprintf(deployingMyFavContractScriptTemplate, "MyFavContract", hex.EncodeToString([]byte(myFavContract))))

}

//go:embed scripts/eventHeavyTransaction.cdc
var eventHeavyScriptTemplate string

func EventHeavyScript(favContractAddress flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(eventHeavyScriptTemplate, favContractAddress))
}

//go:embed scripts/compHeavyTransaction.cdc
var compHeavyScriptTemplate string

func ComputationHeavyScript(favContractAddress flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(compHeavyScriptTemplate, favContractAddress))
}

//go:embed scripts/ledgerHeavyTransaction.cdc
var ledgerHeavyScriptTemplate string

func LedgerHeavyScript(favContractAddress flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(ledgerHeavyScriptTemplate, favContractAddress))
}

//go:embed scripts/execDataHeavyTransaction.cdc
var execDataHeavyScriptTemplate string

func ExecDataHeavyScript(favContractAddress flowsdk.Address) []byte {
	return []byte(fmt.Sprintf(execDataHeavyScriptTemplate, favContractAddress))
}

//go:embed scripts/constExecCostTransaction.cdc
var constExecTransactionTemplate string

func generateRandomStringWithLen(commentLen uint) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := make([]byte, commentLen)
	for i := range bytes {
		bytes[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(bytes)
}

func generateAuthAccountParamList(authAccountNum uint) string {
	authAccountList := []string{}
	for i := uint(0); i < authAccountNum; i++ {
		authAccountList = append(authAccountList, fmt.Sprintf("acct%d: AuthAccount", i+1))
	}
	return strings.Join(authAccountList, ", ")
}

// ConstExecCostTransaction returns a transaction script for constant execution size (0)
func ConstExecCostTransaction(numOfAuthorizer, commentSizeInByte uint) []byte {
	commentStr := generateRandomStringWithLen(commentSizeInByte)
	authAccountListStr := generateAuthAccountParamList(numOfAuthorizer)

	// the transaction template has two `%s`: #1 is for comment; #2 is for AuthAccount param list
	return []byte(fmt.Sprintf(constExecTransactionTemplate, commentStr, authAccountListStr))
}

func bytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewUInt8(b)
	}

	return cadence.NewArray(values)
}

// TODO add tx size heavy similar to add keys
