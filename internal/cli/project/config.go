package project

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dapperlabs/flow-go/pkg/types"
)

const ConfigPath = "flow.json"

type AccountConfig struct {
	Address       string `json:"address"`
	PrivateKeyHex string `json:"privateKey"`
}

func (a *AccountConfig) PrivateKey() (types.AccountPrivateKey, error) {
	privateKeyBytes, err := hex.DecodeString(a.PrivateKeyHex)
	if err != nil {
		return types.AccountPrivateKey{}, err
	}

	privateKey, _ := types.DecodeAccountPrivateKey(privateKeyBytes)
	if err != nil {
		return types.AccountPrivateKey{}, err
	}

	return privateKey, nil
}

type Config struct {
	Accounts map[string]*AccountConfig `json:"accounts"`
}

func SaveConfig(conf *Config) error {
	data, err := json.MarshalIndent(conf, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(ConfigPath, data, 0777)
}

func LoadConfig() *Config {
	f, err := os.Open(ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		fmt.Printf("Failed to open project configuration in %s\n", ConfigPath)
		os.Exit(1)
	}

	d := json.NewDecoder(f)

	var conf Config

	if err := d.Decode(&conf); err != nil {
		fmt.Printf("%s contains invalid json: %s\n", ConfigPath, err.Error())
		os.Exit(1)
	}

	return &conf
}

func ConfigExists() bool {
	info, err := os.Stat(ConfigPath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
