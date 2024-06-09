package diff_states

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	flagOutputDirectory  string
	flagPayloads1        string
	flagPayloads2        string
	flagState1           string
	flagState2           string
	flagStateCommitment1 string
	flagStateCommitment2 string
	flagRaw              bool
	flagNWorker          int
	flagChain            string
)

var Cmd = &cobra.Command{
	Use:   "diff-states",
	Short: "Compares the given states",
	Run:   run,
}

const ReporterName = "state-diff"

func init() {

	// Input 1

	Cmd.Flags().StringVar(
		&flagPayloads1,
		"payloads-1",
		"",
		"Input payload file name 1",
	)

	Cmd.Flags().StringVar(
		&flagState1,
		"state-1",
		"",
		"Input state file name 1",
	)
	Cmd.Flags().StringVar(
		&flagStateCommitment1,
		"state-commitment-1",
		"",
		"Input state commitment 1",
	)

	// Input 2

	Cmd.Flags().StringVar(
		&flagPayloads2,
		"payloads-2",
		"",
		"Input payload file name 2",
	)

	Cmd.Flags().StringVar(
		&flagState2,
		"state-2",
		"",
		"Input state file name 2",
	)

	Cmd.Flags().StringVar(
		&flagStateCommitment2,
		"state-commitment-2",
		"",
		"Input state commitment 2",
	)

	// Other

	Cmd.Flags().StringVar(
		&flagOutputDirectory,
		"output-directory",
		"",
		"Output directory",
	)
	_ = Cmd.MarkFlagRequired("output-directory")

	Cmd.Flags().BoolVar(
		&flagRaw,
		"raw",
		true,
		"Raw or value",
	)

	Cmd.Flags().IntVar(
		&flagNWorker,
		"n-worker",
		10,
		"number of workers to use",
	)

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")
}

func run(*cobra.Command, []string) {

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	_ = chainID.Chain()

	if flagPayloads1 == "" && flagState1 == "" {
		log.Fatal().Msg("Either --payloads-1 or --state-1 must be provided")
	} else if flagPayloads1 != "" && flagState1 != "" {
		log.Fatal().Msg("Only one of --payloads-1 or --state-1 must be provided")
	}
	if flagState1 != "" && flagStateCommitment1 == "" {
		log.Fatal().Msg("--state-commitment-1 must be provided when --state-1 is provided")
	}

	if flagPayloads2 == "" && flagState2 == "" {
		log.Fatal().Msg("Either --payloads-2 or --state-2 must be provided")
	} else if flagPayloads2 != "" && flagState2 != "" {
		log.Fatal().Msg("Only one of --payloads-2 or --state-2 must be provided")
	}
	if flagState2 != "" && flagStateCommitment2 == "" {
		log.Fatal().Msg("--state-commitment-2 must be provided when --state-2 is provided")
	}

	rw := reporters.NewReportFileWriterFactory(flagOutputDirectory, log.Logger).
		ReportWriter(ReporterName)
	defer rw.Close()

	var registers1, registers2 *registers.ByAccount
	{
		// Load payloads and create registers.
		// Define in a block, so that the memory is released after the registers are created.
		payloads1, payloads2 := loadPayloads()

		payloadCount1 := len(payloads1)
		payloadCount2 := len(payloads2)
		if payloadCount1 != payloadCount2 {
			log.Warn().Msgf(
				"Payloads files have different number of payloads: %d vs %d",
				payloadCount1,
				payloadCount2,
			)
		}

		registers1, registers2 = payloadsToRegisters(payloads1, payloads2)

		accountCount1 := registers1.AccountCount()
		accountCount2 := registers2.AccountCount()
		if accountCount1 != accountCount2 {
			log.Warn().Msgf(
				"Registers have different number of accounts: %d vs %d",
				accountCount1,
				accountCount2,
			)
		}
	}

	diff(registers1, registers2, chainID, rw)
}

func loadPayloads() (payloads1, payloads2 []*ledger.Payload) {

	log.Info().Msg("Loading payloads")

	var group errgroup.Group

	group.Go(func() (err error) {
		if flagPayloads1 != "" {
			_, payloads1, err = util.ReadPayloadFile(log.Logger, flagPayloads1)
		} else {
			log.Info().Msg("Reading first trie")

			stateCommitment := parseStateCommitment(flagStateCommitment1)
			payloads1, err = readTrie(flagState1, stateCommitment)
		}
		return
	})

	group.Go(func() (err error) {
		if flagPayloads2 != "" {
			_, payloads2, err = util.ReadPayloadFile(log.Logger, flagPayloads2)
		} else {
			log.Info().Msg("Reading second trie")

			stateCommitment := parseStateCommitment(flagStateCommitment2)
			payloads2, err = readTrie(flagState2, stateCommitment)
		}
		return
	})

	err := group.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read payloads")
	}

	log.Info().Msg("Finished loading payloads")

	return
}

func payloadsToRegisters(payloads1, payloads2 []*ledger.Payload) (registers1, registers2 *registers.ByAccount) {

	log.Info().Msg("Creating registers from payloads")

	var group errgroup.Group

	group.Go(func() (err error) {
		log.Info().Msgf("Creating registers from first payloads (%d)", len(payloads1))

		registers1, err = registers.NewByAccountFromPayloads(payloads1)

		log.Info().Msgf(
			"Created %d registers from payloads (%d accounts)",
			registers1.Count(),
			registers1.AccountCount(),
		)

		return
	})

	group.Go(func() (err error) {
		log.Info().Msgf("Creating registers from second payloads (%d)", len(payloads2))

		registers2, err = registers.NewByAccountFromPayloads(payloads2)

		log.Info().Msgf(
			"Created %d registers from payloads (%d accounts)",
			registers2.Count(),
			registers2.AccountCount(),
		)

		return
	})

	err := group.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create registers from payloads")
	}

	log.Info().Msg("Finished creating registers from payloads")

	return
}

var accountsDiffer = errors.New("accounts differ")

func diff(
	registers1 *registers.ByAccount,
	registers2 *registers.ByAccount,
	chainID flow.ChainID,
	rw reporters.ReportWriter,
) {
	log.Info().Msg("Diffing accounts")

	err := registers1.ForEachAccount(func(accountRegisters1 *registers.AccountRegisters) (err error) {
		owner := accountRegisters1.Owner()

		if !registers2.HasAccountOwner(owner) {
			rw.Write(accountMissing{
				Owner: owner,
				State: 2,
			})

			return nil
		}

		accountRegisters2 := registers2.AccountRegisters(owner)

		if accountRegisters1.Count() != accountRegisters2.Count() {
			rw.Write(countDiff{
				Owner:  owner,
				State1: accountRegisters1.Count(),
				State2: accountRegisters2.Count(),
			})
		}

		err = accountRegisters1.ForEach(func(owner, key string, value1 []byte) error {
			var value2 []byte
			value2, err = accountRegisters2.Get(owner, key)
			if err != nil {
				return err
			}

			if !bytes.Equal(value1, value2) {

				if flagRaw {
					rw.Write(rawDiff{
						Owner:  owner,
						Key:    key,
						Value1: value1,
						Value2: value2,
					})
				} else {
					// stop on first difference in accounts
					return accountsDiffer
				}
			}

			return nil
		})
		if err != nil {
			if flagRaw || !errors.Is(err, accountsDiffer) {
				return err
			}

			address, err := common.BytesToAddress([]byte(owner))
			if err != nil {
				return err
			}

			migrations.NewCadenceValueDiffReporter(
				address,
				chainID,
				rw,
				true,
				flagNWorker,
			).DiffStates(
				accountRegisters1,
				accountRegisters2,
				migrations.AllStorageMapDomains,
			)
		}

		return nil
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to diff")
	}

	err = registers2.ForEachAccount(func(accountRegisters2 *registers.AccountRegisters) (err error) {
		owner := accountRegisters2.Owner()

		if !registers1.HasAccountOwner(owner) {
			rw.Write(accountMissing{
				Owner: owner,
				State: 1,
			})
			return nil
		}

		return nil
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to diff")
	}

	log.Info().Msg("Finished diffing accounts")

}

func readTrie(dir string, targetHash flow.StateCommitment) ([]*ledger.Payload, error) {
	log.Info().Msg("init WAL")

	diskWal, err := wal.NewDiskWAL(
		log.Logger,
		nil,
		metrics.NewNoopCollector(),
		dir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create disk WAL: %w", err)
	}

	log.Info().Msg("init ledger")

	led, err := complete.NewLedger(
		diskWal,
		complete.DefaultCacheSize,
		&metrics.NoopCollector{},
		log.Logger,
		complete.DefaultPathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	log.Info().Msg("init compactor")

	compactor, err := complete.NewCompactor(
		led,
		diskWal,
		log.Logger,
		complete.DefaultCacheSize,
		checkpointDistance,
		checkpointsToKeep,
		atomic.NewBool(false),
		&metrics.NoopCollector{},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create compactor: %w", err)
	}

	log.Info().Msgf("waiting for compactor to load checkpoint and WAL")

	<-compactor.Ready()

	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()

	state := ledger.State(targetHash)

	trie, err := led.Trie(ledger.RootHash(state))
	if err != nil {
		s, _ := led.MostRecentTouchedState()
		log.Info().
			Str("hash", s.String()).
			Msgf("Most recently touched state")
		return nil, fmt.Errorf("cannot get trie at the given state commitment: %w", err)
	}

	return trie.AllPayloads(), nil
}

func parseStateCommitment(stateCommitmentHex string) flow.StateCommitment {
	var err error
	stateCommitmentBytes, err := hex.DecodeString(stateCommitmentHex)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get decode the state commitment")
	}

	stateCommitment, err := flow.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid state commitment length")
	}

	return stateCommitment
}

type rawDiff struct {
	Owner  string
	Key    string
	Value1 []byte
	Value2 []byte
}

var _ json.Marshaler = rawDiff{}

func (e rawDiff) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string `json:"kind"`
		Owner  string `json:"owner"`
		Key    string `json:"key"`
		Value1 string `json:"value1"`
		Value2 string `json:"value2"`
	}{
		Kind:   "raw-diff",
		Owner:  hex.EncodeToString([]byte(e.Owner)),
		Key:    hex.EncodeToString([]byte(e.Key)),
		Value1: hex.EncodeToString(e.Value1),
		Value2: hex.EncodeToString(e.Value2),
	})
}

type accountMissing struct {
	Owner string
	State int
}

var _ json.Marshaler = accountMissing{}

func (e accountMissing) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string `json:"kind"`
		Owner string `json:"owner"`
		State int    `json:"state"`
	}{
		Kind:  "account-missing",
		Owner: hex.EncodeToString([]byte(e.Owner)),
		State: e.State,
	})
}

type countDiff struct {
	Owner  string
	State1 int
	State2 int
}

var _ json.Marshaler = countDiff{}

func (e countDiff) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string `json:"kind"`
		Owner  string `json:"owner"`
		State1 int    `json:"state1"`
		State2 int    `json:"state2"`
	}{
		Kind:   "count-diff",
		Owner:  hex.EncodeToString([]byte(e.Owner)),
		State1: e.State1,
		State2: e.State2,
	})
}
