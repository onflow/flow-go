package create

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"log"

	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

type Config struct {
	Signer string   `default:"root" flag:"signer,s"`
	Keys   []string `flag:"key,k"`
	Code   string   `flag:"code,c"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account",
	Run: func(cmd *cobra.Command, args []string) {
		projectConf := project.LoadConfig()

		signer := projectConf.Accounts[conf.Signer]
		signerAddr := types.HexToAddress(signer.Address)
		signerKey, err := signer.PrivateKey()
		if err != nil {
			utils.Exit(1, "Failed to load signer key")
		}

		accountKeys := make([]types.AccountPublicKey, len(conf.Keys))

		for i, privateKeyHex := range conf.Keys {
			prKeyDer, err := hex.DecodeString(privateKeyHex)
			if err != nil {
				utils.Exit(1, "Failed to decode private key")
			}

			privateKey, err := types.DecodeAccountPrivateKey(prKeyDer)
			if err != nil {
				utils.Exit(1, "Failed to decode private key")
			}

			accountKeys[i] = privateKey.PublicKey(constants.AccountKeyWeightThreshold)
		}

		var code []byte

		if conf.Code != "" {
			code, err = ioutil.ReadFile(conf.Code)
			if err != nil {
				utils.Exitf(1, "Failed to parse Cadence code from %s", conf.Code)
			}
		}

		script, err := templates.CreateAccount(accountKeys, code)
		if err != nil {
			utils.Exit(1, "Failed to generate transaction script")
		}

		tx := types.Transaction{
			Script:       script,
			Nonce:        1,
			ComputeLimit: 10,
			PayerAccount: signerAddr,
		}

		sig, err := keys.SignTransaction(tx, signerKey)
		if err != nil {
			utils.Exit(1, "Failed to sign transaction")
		}

		tx.AddSignature(signerAddr, sig)

		client, err := client.New("localhost:5000")
		if err != nil {
			utils.Exit(1, "Failed to connect to emulator")
		}

		err = client.SendTransaction(context.Background(), tx)
		if err != nil {
			utils.Exit(1, "Failed to send account creation transaction")
		}
	},
}

func init() {
	initConfig()
}

func initConfig() {
	err := sconfig.New(&conf).
		BindFlags(Cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}
}
