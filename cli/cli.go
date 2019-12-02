// Package cli defines constants, configurations, and utilities that are used
// across the Flow CLI.
package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	EnvPrefix = "FLOW"
)

var (
	ConfigPath = "flow.json"
)

func Exit(code int, msg string) {
	fmt.Println(msg)
	os.Exit(code)
}

func Exitf(code int, msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
	os.Exit(code)
}

func DecodeAccountPrivateKeyHex(prKeyHex string) (flow.AccountPrivateKey, error) {
	prKeyBytes, err := hex.DecodeString(prKeyHex)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}
	prKey, err := flow.DecodeAccountPrivateKey(prKeyBytes)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}
	return prKey, nil
}

func MustDecodeAccountPrivateKeyHex(prKeyHex string) flow.AccountPrivateKey {
	prKey, err := DecodeAccountPrivateKeyHex(prKeyHex)
	if err != nil {
		Exitf(1, "Failed to decode account private key err: %v", err)
	}
	return prKey
}

func RandomSeed(n int) []byte {
	seed := make([]byte, n)

	_, err := rand.Read(seed)
	if err != nil {
		Exitf(1, "Failed to generate random seed: %v", err)
	}

	return seed
}
