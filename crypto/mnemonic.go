package crypto

import (
	"io/ioutil"
	"os"
)

const (
	MnemonicFile = "bamboo.mnemonic"
)

func LoadMnemonic() (string, error) {
	file, err := os.Open(MnemonicFile)
	if err != nil {
		return "", &FileDoesNotExistError{filename: MnemonicFile}
	}
	defer file.Close()

	contents, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}
	mnemonic := string(contents)
	return mnemonic, nil
}
