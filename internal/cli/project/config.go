package project

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/types"
)

const (
	ConfigPath = "flow.json"
)

var (
	RootName    = "root"
	RootAddress = types.HexToAddress("01")
)

type Account struct {
	Address    types.Address
	PrivateKey types.AccountPrivateKey
}

// An internal utility struct that defines how Account is converted to JSON.
type accountJSON struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
}

func (acct *Account) MarshalJSON() ([]byte, error) {
	prKeyBytes, err := types.EncodeAccountPrivateKey(acct.PrivateKey)
	if err != nil {
		return nil, err
	}

	prKeyHex := hex.EncodeToString(prKeyBytes)
	return json.Marshal(accountJSON{
		Address:    acct.Address.Hex(),
		PrivateKey: prKeyHex,
	})
}

func (acct *Account) UnmarshalJSON(data []byte) (err error) {
	var alias accountJSON
	if err = json.Unmarshal(data, &alias); err != nil {
		return
	}

	var prKeyBytes []byte
	prKeyBytes, err = hex.DecodeString(alias.PrivateKey)
	if err != nil {
		return
	}

	acct.Address = types.HexToAddress(alias.Address)
	acct.PrivateKey, err = types.DecodeAccountPrivateKey(prKeyBytes)
	return
}

type Config struct {
	Accounts map[string]*Account `json:"accounts"`
}

func NewConfig() *Config {
	return &Config{
		Accounts: make(map[string]*Account),
	}
}

func (c *Config) RootAccount() *Account {
	rootAcct, ok := c.Accounts[RootName]
	if !ok {
		utils.Exit(1, "Missing root account!")
	}
	return rootAcct
}

func (c *Config) SetRootAccount(prKey types.AccountPrivateKey) {
	c.Accounts[RootName] = &Account{
		Address:    RootAddress,
		PrivateKey: prKey,
	}
}

func SaveConfig(conf *Config) error {
	data, err := json.MarshalIndent(conf, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(ConfigPath, data, 0777)
}

func MustSaveConfig(conf *Config) {
	if err := SaveConfig(conf); err != nil {
		utils.Exitf(1, "Failed to save config err: ", err)
	}
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
