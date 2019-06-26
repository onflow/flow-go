package client

import (
	"os"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// InitClient sets up a mnemonic file for the user to start the Bamboo Emulator.
func InitClient(log *logrus.Logger, reset bool) {
	if !fileExists(crypto.MnemonicFile) || reset {
		file, err := os.Create(crypto.MnemonicFile)
		if err != nil {
			log.WithError(err).Fatal("Failed to create mnemonic file")
		}
		defer file.Close()

		wallet, err := crypto.CreateWallet("BAMBOO")
		if err != nil {
			log.WithError(err).Fatal("Failed to generate HD wallet")
		}

		_, err = file.Write([]byte(wallet.Mnemonic))
		if err != nil {
			log.WithError(err).Fatal("Failed to save mnemonic")
		}
	}
	log.Info("Bamboo Client setup finished! Begin by running: bamboo emulator start")
}

func fileExists(filename string) bool {
    info, err := os.Stat(filename)
    if os.IsNotExist(err) {
        return false
    }
    return !info.IsDir()
}
