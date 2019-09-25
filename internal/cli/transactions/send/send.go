package send

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/dapperlabs/flow-go/internal/cli/project"
	"github.com/dapperlabs/flow-go/internal/cli/utils"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/psiemens/sconfig"
	"github.com/spf13/cobra"
)

type Config struct {
	Signer string `default:"root" flag:"signer,s"`
	Code   string `flag:"code,c"`
	Nonce  uint64 `flag:"nonce,n"`
}

var conf Config

var Cmd = &cobra.Command{
	Use:   "send",
	Short: "Send a transaction",
	Run: func(cmd *cobra.Command, args []string) {
		projectConf := project.LoadConfig()

		signer := projectConf.Accounts[conf.Signer]
		signerAddr := types.HexToAddress(signer.Address)
		signerKey, err := utils.DecodePrivateKey(signer.PrivateKey)
		if err != nil {
			utils.Exit("Failed to load signer key", 1)
		}

		var code []byte

		if conf.Code != "" {
			code, err = ioutil.ReadFile(conf.Code)
			if err != nil {
				utils.Exit(fmt.Sprintf("Failed to load BPL code from %s", conf.Code), 1)
			}
		}

		tx := types.RawTransaction{
			Script:       code,
			Nonce:        conf.Nonce,
			ComputeLimit: 10,
		}
		signedTx, err := tx.SignPayer(signerAddr, signerKey)
		if err != nil {
			utils.Exit("Failed to sign transaction", 1)
		}

		client, err := client.New("localhost:5000")
		if err != nil {
			utils.Exit("Failed to connect to emulator", 1)
		}

		err = client.SendTransaction(context.Background(), *signedTx)
		if err != nil {
			utils.Exit("Failed to send transaction", 1)
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
