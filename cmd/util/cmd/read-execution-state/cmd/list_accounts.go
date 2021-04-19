package cmd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	executionState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/model/flow"
)

var listAccountsCmd = &cobra.Command{
	Use:   "list-accounts",
	Short: "lists accounts on given chain (from first to address generator state saved)",
	Run:   listAccounts,
}

var (
	flagStateCommitment string
	flagChain           string
)

func init() {
	RootCmd.AddCommand(listAccountsCmd)

	listAccountsCmd.Flags().StringVarP(&flagStateCommitment, "state", "s", "", "the state commitment (64 chars, hex-encoded)")
	_ = listAccountsCmd.MarkFlagRequired("state")

	listAccountsCmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = listAccountsCmd.MarkFlagRequired("chain")
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

func listAccounts(*cobra.Command, []string) {
	invoked := time.Now()

	forest, err := initForest()
	if err != nil {
		log.Fatal().Err(err).Msg("error while loading execution state")
	}

	stateCommitment, err := hex.DecodeString(flagStateCommitment)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid flag, cannot decode")
	}

	if len(stateCommitment) != 32 {
		log.Fatal().Err(err).Int("recieved", len(stateCommitment)).Int("expected", 32).
			Msgf("invalid number of bytes for state commitment")
	}

	chain, err := getChain(flagChain)
	if err != nil {
		log.Fatal().Err(err).Msgf("invalid chain name")
	}

	view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {

		ledgerKey := executionState.RegisterIDToKey(flow.NewRegisterID(owner, controller, key))
		path, err := pathfinder.KeyToPath(ledgerKey, 0)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot convert key to path")
		}

		read := &ledger.TrieRead{
			RootHash: stateCommitment,
			Paths: []ledger.Path{
				path,
			},
		}

		payload, err := forest.Read(read)
		if err != nil {
			return nil, err
		}

		return payload[0].Value, nil
	})

	stateHolder := state.NewStateHolder(state.NewState(view))
	accounts := state.NewAccounts(stateHolder)
	finalGenerator := state.NewStateBoundAddressGenerator(stateHolder, chain)

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

		account, err := accounts.Get(address)
		if err != nil {
			log.Fatal().Err(err).Msg("error while getting account")
		}
		log.Info().Msgf("Address: %v", address.Short())

		b, err := json.MarshalIndent(account, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("error while marshalling account")
		}
		fmt.Println(string(b))
	}

	log.Info().Float64("time_elapsed_s", time.Since(invoked).Seconds()).Msg("completed listing accounts")
}
