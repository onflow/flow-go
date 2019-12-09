package config

import (
	"errors"

	"github.com/dapperlabs/flow-go/sdk/keys"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Config defines configuration for the Language Server. These options are
// determined by the client and passed to the server at initialization.
type Config struct {
	// The key of the root account.
	RootAccountKey flow.AccountPrivateKey `json:"rootAccountKey"`
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

	rootAccountKeyHex, ok := optsMap["rootAccountKey"].(string)
	if !ok {
		return Config{}, errors.New("missing rootAccountKey field")
	}
	emulatorAddr, ok := optsMap["emulatorAddress"].(string)
	if !ok {
		return Config{}, errors.New("missing emulatorAddress field")
	}

	conf.RootAccountKey, err = keys.DecodePrivateKeyHex(rootAccountKeyHex)
	if err != nil {
		return
	}
	conf.EmulatorAddr = emulatorAddr

	return
}
