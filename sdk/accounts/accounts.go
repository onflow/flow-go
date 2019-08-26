package accounts

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// CreateAccount generates a transaction that creates a new account.
func CreateAccount(publicKey, code []byte) *types.RawTransaction {
	publicKeyStr := bytesToString(publicKey)
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main(account: Account) {
			let publicKey = %s
			let code = %s
			createAccount(publicKey, code)
		}
	`, publicKeyStr, codeStr)

	return &types.RawTransaction{
		Script: []byte(script),
	}
}

// UpdateAccountCode generates a transaction that updates the code associated with an account.
func UpdateAccountCode(account types.Address, code []byte) *types.RawTransaction {
	accountStr := bytesToString(account.Bytes())
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main(account: Account) {
			let account = %s
			let code = %s
			updateAccountCode(account, code)
		}
	`, accountStr, codeStr)

	return &types.RawTransaction{
		Script: []byte(script),
	}
}

// bytesToString converts a byte slice to a comma-separted list of uint8 integers.
func bytesToString(b []byte) string {
	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}
