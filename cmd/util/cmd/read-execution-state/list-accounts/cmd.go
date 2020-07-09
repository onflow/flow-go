package list_accounts

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/fvm"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie"
)

var cmd = &cobra.Command{
	Use:   "list-accounts",
	Short: "Lists accounts on given chain (from first to address generator state saved)",
	Run:   run,
}

var stateLoader func() *mtrie.MForest = nil
var flagStateCommitment string
var flagChain string

func Init(f func() *mtrie.MForest) *cobra.Command {
	stateLoader = f

	cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"State commitment (64 chars, hex-encoded)")
	_ = cmd.MarkFlagRequired("state-commitment")

	cmd.Flags().StringVar(&flagChain, "chain", "",
		"Chain name")
	_ = cmd.MarkFlagRequired("chain")

	return cmd
}

func getChain(chainName string) (chain flow.Chain, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid chain: %s", r)
		}
	}()
	chain = flow.ChainID(chainName).Chain()
	return
}

func run(*cobra.Command, []string) {
	startTime := time.Now()

	mForest := stateLoader()

	stateCommitment, err := hex.DecodeString(flagStateCommitment)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid flag, cannot decode")
	}

	if len(stateCommitment) != 32 {
		log.Fatal().Err(err).Msgf("invalid number of bytes, got %d expected %d", len(stateCommitment), 32)
	}

	chain, err := getChain(flagChain)
	if err != nil {
		log.Fatal().Err(err).Msgf("invalid chain name")
	}

	ledger := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
		values, err := mForest.Read(stateCommitment, [][]byte{key})
		if err != nil {
			return nil, err
		}
		return values[0], nil
	})

	dal := fvm.NewLedgerDAL(ledger, chain)

	finalGenerator, err := dal.GetAddressState()
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot get current address state")
	}

	finalState := finalGenerator.Bytes()

	generator := chain.NewAddressGenerator()

	for !bytes.Equal(generator.Bytes(), finalState) {
		address, err := generator.NextAddress()
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot get address")
		}

		account := dal.GetAccount(address)
		fmt.Printf("Account address %s:\n", account.Address.Short())
		b, err := json.MarshalIndent(account, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("error while marshalling account")
		}
		fmt.Println(string(b))
	}

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
