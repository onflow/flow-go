package config

import (
	"errors"

	"github.com/dapperlabs/flow-go/sdk/keys"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Config defines configuration for the Language Server. These options are
// determined by the client and passed to the server at initialization.
type Config struct {
	// The key and address of the account to use when sending transactions.
	AccountKey  flow.AccountPrivateKey `json:"accountKey"`
	AccountAddr flow.Address           `json:"accountAddress"`
	// The address where the emulator is running.
	EmulatorAddr string `json:"emulatorAddress"`
}

// FromInitializationOptions creates a new config instance from the
// initialization options field passed from the client at startup.
//
// Returns an error if any fields are missing or malformed.
func FromInitializationOptions(opts interface{}) (conf Config, err error) {
	optsMap, ok := opts.(map[string]interface{})
	if !ok {
		return Config{}, errors.New("")
	}

	accountKeyHex, ok := optsMap["accountKey"].(string)
	if !ok {
		return Config{}, errors.New("missing accountKey field")
	}
	accountAddrHex, ok := optsMap["accountAddress"].(string)
	if !ok {
		return Config{}, errors.New("missing accountAddress field")
	}
	emulatorAddr, ok := optsMap["emulatorAddress"].(string)
	if !ok {
		return Config{}, errors.New("missing emulatorAddress field")
	}

	conf.AccountKey, err = keys.DecodePrivateKeyHex(accountKeyHex)
	if err != nil {
		return
	}
	conf.AccountAddr = flow.HexToAddress(accountAddrHex)
	conf.EmulatorAddr = emulatorAddr

	return
}
