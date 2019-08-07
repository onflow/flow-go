package client

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

type AccountConfig struct {
	Address string `json:"account"`
	Seed    string `json:"seed"`
}

func LoadAccountFromFile(filename string) (*types.AccountKey, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	return LoadAccount(f)
}

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

func bytesToString(b []byte) string {
	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}
