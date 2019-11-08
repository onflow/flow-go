package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dapperlabs/flow-go/model/flow"
)

var (
	RootAccountName    = "root"
	RootAccountAddress = flow.HexToAddress("01")
)

type Account struct {
	Address    flow.Address
	PrivateKey flow.AccountPrivateKey
}

// An internal utility struct that defines how Account is converted to JSON.
type accountJSON struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
}

func (acct *Account) MarshalJSON() ([]byte, error) {
	prKeyBytes, err := flow.EncodeAccountPrivateKey(acct.PrivateKey)
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

	acct.Address = flow.HexToAddress(alias.Address)
	acct.PrivateKey, err = flow.DecodeAccountPrivateKey(prKeyBytes)
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
	rootAcct, ok := c.Accounts[RootAccountName]
	if !ok {
		Exit(1, "Missing root account!")
	}
	return rootAcct
}

func (c *Config) SetRootAccount(prKey flow.AccountPrivateKey) {
	c.Accounts[RootAccountName] = &Account{
		Address:    RootAccountAddress,
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
		Exitf(1, "Failed to save config err: %v", err)
	}
}

func LoadConfig() *Config {
	f, err := os.Open(ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Emulator config file %s does not exist. Please initialize first\n", ConfigPath)
		} else {
			fmt.Printf("Failed to open project configuration in %s\n", ConfigPath)
		}

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
