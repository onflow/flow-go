package project

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

const ConfigPath = "bamboo.json"

type AccountConfig struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
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
