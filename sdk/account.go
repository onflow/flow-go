package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// AccountConfig is the configuration format for a user account.
//
// This structure is used to load account configuration from JSON.
type AccountConfig struct {
	Address string `json:"account"`
	Seed    string `json:"seed"`
}

// LoadAccountFromFile loads an account key from a JSON file.
//
// An error will be returned if the file cannot be read or if it contains invalid JSON.
func LoadAccountFromFile(filename string) (*types.AccountKey, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	return LoadAccount(f)
}

// LoadAccount parses an account key from a JSON reader.
//
// An error will be returned if the reader contains invalid JSON.
func LoadAccount(r io.Reader) (*types.AccountKey, error) {
	d := json.NewDecoder(r)

	var conf AccountConfig

	if err := d.Decode(&conf); err != nil {
		return nil, err
	}

	keyPair, err := crypto.GenerateKeyPair(conf.Seed)
	if err != nil {
		return nil, err
	}

	return &types.AccountKey{
		Account: types.HexToAddress(conf.Address),
		KeyPair: keyPair,
	}, nil
}

// CreateAccount generates a transaction that creates a new account.
func CreateAccount(publicKey, code []byte) *types.RawTransaction {
	publicKeyStr := bytesToString(publicKey)
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main() {
			let publicKey = %s
			let code = %s
			createAccount(publicKey, code)
		}
	`, publicKeyStr, codeStr)

	return &types.RawTransaction{
		Script:    []byte(script),
		Timestamp: time.Now(),
	}
}

// UpdateAccountCode generates a transaction that updates the code associated with an account.
func UpdateAccountCode(account types.Address, code []byte) *types.RawTransaction {
	accountStr := bytesToString(account.Bytes())
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main() {
			let account = %s
			let code = %s
			updateAccountCode(account, code)
		}
	`, accountStr, codeStr)

	return &types.RawTransaction{
		Script:    []byte(script),
		Timestamp: time.Now(),
	}
}

// bytesToString converts a byte slice to a comma-separted list of uint8 integers.
func bytesToString(b []byte) string {
	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}
