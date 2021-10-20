package list_accounts

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
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/model/flow"
)

var cmd = &cobra.Command{
	Use:   "list-accounts",
	Short: "Lists accounts on given chain (from first to address generator state saved)",
	Run:   run,
}

var stateLoader func() *mtrie.Forest = nil
var flagStateCommitment string
var flagChain string

func Init(f func() *mtrie.Forest) *cobra.Command {
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

	forest := stateLoader()

	stateCommitmentBytes, err := hex.DecodeString(flagStateCommitment)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid flag, cannot decode")
	}

	stateCommitment, err := flow.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		log.Fatal().Err(err).Msgf("invalid number of bytes, got %d expected %d", len(stateCommitmentBytes), len(stateCommitment))
	}

	chain, err := getChain(flagChain)
	if err != nil {
		log.Fatal().Err(err).Msgf("invalid chain name")
	}

	ldg := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {

		ledgerKey := executionState.RegisterIDToKey(flow.NewRegisterID(owner, controller, key))
		path, err := pathfinder.KeyToPath(ledgerKey, complete.DefaultPathFinderVersion)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot convert key to path")
		}

		read := &ledger.TrieRead{
			RootHash: ledger.RootHash(stateCommitment),
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

	sth := state.NewStateHolder(state.NewState(ldg, state.NewInteractionLimiter()))
	accounts := state.NewAccounts(sth)
	finalGenerator := state.NewStateBoundAddressGenerator(sth, chain)
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
