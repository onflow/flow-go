package utils

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/dapperlabs/flow-go/pkg/types"
)

func Exit(code int, msg string) {
	fmt.Println(msg)
	os.Exit(code)
}

func Exitf(code int, msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
	os.Exit(code)
}

func DecodeAccountPrivateKeyHex(prKeyHex string) (types.AccountPrivateKey, error) {
	prKeyBytes, err := hex.DecodeString(prKeyHex)
	if err != nil {
		return types.AccountPrivateKey{}, err
	}
	prKey, err := types.DecodeAccountPrivateKey(prKeyBytes)
	if err != nil {
		return types.AccountPrivateKey{}, err
	}
	return prKey, nil
}

func MustDecodeAccountPrivateKeyHex(prKeyHex string) types.AccountPrivateKey {
	prKey, err := DecodeAccountPrivateKeyHex(prKeyHex)
	if err != nil {
		Exitf(1, "Failed to decode account private key err: %v", err)
	}
	return prKey
}
