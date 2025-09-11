package diffkeys

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/onflow/cadence/common"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	moduleUtil "github.com/onflow/flow-go/module/util"
)

var (
	flagOutputDirectory   string
	flagPayloadsV3        string
	flagPayloadsV4        string
	flagStateV3           string
	flagStateV4           string
	flagStateCommitmentV3 string
	flagStateCommitmentV4 string
	flagNWorker           int
	flagChain             string
)

var Cmd = &cobra.Command{
	Use:   "diff-keys",
	Short: "Compare account public keys in the given state-v3 with state-v4 and output to JSONL file (empty file if no difference)",
	Run:   run,
}

const ReporterName = "key-diff"

type stateType uint8

const (
	oldState stateType = 1
	newState stateType = 2
)

func init() {

	// Input with account public keys in v3 format

	Cmd.Flags().StringVar(
		&flagPayloadsV3,
		"payloads-v3",
		"",
		"Input payload file name with account public keys in v3 format",
	)

	Cmd.Flags().StringVar(
		&flagStateV3,
		"state-v3",
		"",
		"Input state file name with account public keys in v3 format",
	)
	Cmd.Flags().StringVar(
		&flagStateCommitmentV3,
		"state-commitment-v3",
		"",
		"Input state commitment for state-v3",
	)

	// Input with account public keys in v4 format

	Cmd.Flags().StringVar(
		&flagPayloadsV4,
		"payloads-v4",
		"",
		"Input payload file name with account public keys in v4 format",
	)

	Cmd.Flags().StringVar(
		&flagStateV4,
		"state-v4",
		"",
		"Input state file name with account public keys in v4 format",
	)

	Cmd.Flags().StringVar(
		&flagStateCommitmentV4,
		"state-commitment-v4",
		"",
		"Input state commitment for state-v4",
	)

	// Other

	Cmd.Flags().StringVar(
		&flagOutputDirectory,
		"output-directory",
		"",
		"Output directory",
	)
	_ = Cmd.MarkFlagRequired("output-directory")

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

	if flagPayloadsV3 == "" && flagStateV3 == "" {
		log.Fatal().Msg("Either --payloads-v3 or --state-v3 must be provided")
	} else if flagPayloadsV3 != "" && flagStateV3 != "" {
		log.Fatal().Msg("Only one of --payloads-v4 or --state-v4 must be provided")
	}
	if flagStateV3 != "" && flagStateCommitmentV3 == "" {
		log.Fatal().Msg("--state-commitment-v3 must be provided when --state-v3 is provided")
	}

	if flagPayloadsV4 == "" && flagStateV4 == "" {
		log.Fatal().Msg("Either --payloads-v4 or --state-v4 must be provided")
	} else if flagPayloadsV4 != "" && flagStateV4 != "" {
		log.Fatal().Msg("Only one of --payloads-v4 or --state-v4 must be provided")
	}
	if flagStateV4 != "" && flagStateCommitmentV4 == "" {
		log.Fatal().Msg("--state-commitment-v4 must be provided when --state-v4 is provided")
	}

	rw := reporters.NewReportFileWriterFactoryWithFormat(flagOutputDirectory, log.Logger, reporters.ReportFormatJSONL).
		ReportWriter(ReporterName)
	defer rw.Close()

	var registersV3, registersV4 *registers.ByAccount
	{
		// Load payloads and create registers.
		// Define in a block, so that the memory is released after the registers are created.
		payloadsV3, payloadsV4 := loadPayloads()

		registersV3, registersV4 = payloadsToRegisters(payloadsV3, payloadsV4)

		accountCountV3 := registersV3.AccountCount()
		accountCountV4 := registersV4.AccountCount()
		if accountCountV3 != accountCountV4 {
			log.Warn().Msgf(
				"Registers have different number of accounts: %d vs %d",
				accountCountV3,
				accountCountV4,
			)
		}
	}

	err := diff(registersV3, registersV4, chainID, rw, flagNWorker)
	if err != nil {
		log.Warn().Err(err).Msgf("failed to diff registers")
	}
}

func loadPayloads() (payloads1, payloads2 []*ledger.Payload) {

	log.Info().Msg("Loading payloads")

	var group errgroup.Group

	group.Go(func() (err error) {
		if flagPayloadsV3 != "" {
			log.Info().Msgf("Loading v3 payloads from file at %v", flagPayloadsV3)

			_, payloads1, err = util.ReadPayloadFile(log.Logger, flagPayloadsV3)
			if err != nil {
				err = fmt.Errorf("failed to load v3 payload file: %w", err)
			}
		} else {
			log.Info().Msgf("Reading v3 trie with state commitement %s", flagStateCommitmentV3)

			stateCommitment := util.ParseStateCommitment(flagStateCommitmentV3)
			payloads1, err = util.ReadTrieForPayloads(flagStateV3, stateCommitment)
			if err != nil {
				err = fmt.Errorf("failed to load v3 trie: %w", err)
			}
		}
		return
	})

	group.Go(func() (err error) {
		if flagPayloadsV4 != "" {
			log.Info().Msgf("Loading v4 payloads from file at %v", flagPayloadsV4)

			_, payloads2, err = util.ReadPayloadFile(log.Logger, flagPayloadsV4)
			if err != nil {
				err = fmt.Errorf("failed to load v4 payload file: %w", err)
			}
		} else {
			log.Info().Msgf("Reading v4 trie with state commitment %s", flagStateCommitmentV4)

			stateCommitment := util.ParseStateCommitment(flagStateCommitmentV4)
			payloads2, err = util.ReadTrieForPayloads(flagStateV4, stateCommitment)
			if err != nil {
				err = fmt.Errorf("failed to load v4 trie: %w", err)
			}
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
		log.Info().Msgf("Creating registers from v3 payloads (%d)", len(payloads1))

		registers1, err = registers.NewByAccountFromPayloads(payloads1)
		if err != nil {
			return fmt.Errorf("failed to create registers from v3 payloads: %w", err)
		}

		log.Info().Msgf(
			"Created %d registers from payloads (%d accounts)",
			registers1.Count(),
			registers1.AccountCount(),
		)

		return
	})

	group.Go(func() (err error) {
		log.Info().Msgf("Creating registers from v4 payloads (%d)", len(payloads2))

		registers2, err = registers.NewByAccountFromPayloads(payloads2)
		if err != nil {
			return fmt.Errorf("failed to create registers from v4 payloads: %w", err)
		}

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

func diff(
	registersV3 *registers.ByAccount,
	registersV4 *registers.ByAccount,
	chainID flow.ChainID,
	rw reporters.ReportWriter,
	nWorkers int,
) error {
	log.Info().Msgf("Diffing accounts: v3 count %d, v4 count %d", registersV3.AccountCount(), registersV4.AccountCount())

	if registersV3.AccountCount() < nWorkers {
		nWorkers = registersV3.AccountCount()
	}

	logAccount := moduleUtil.LogProgress(
		log.Logger,
		moduleUtil.DefaultLogProgressConfig(
			"processing account group",
			registersV3.AccountCount(),
		),
	)

	if nWorkers <= 1 {
		foundAccountCountInRegistersV4 := 0

		_ = registersV3.ForEachAccount(func(accountRegistersV3 *registers.AccountRegisters) (err error) {
			owner := accountRegistersV3.Owner()

			if !registersV4.HasAccountOwner(owner) {
				rw.Write(accountMissing{
					Owner: owner,
					State: int(newState),
				})

				return nil
			}

			foundAccountCountInRegistersV4++

			accountRegistersV4 := registersV4.AccountRegisters(owner)

			err = diffAccount(
				owner,
				accountRegistersV3,
				accountRegistersV4,
				chainID,
				rw,
			)
			if err != nil {
				log.Warn().Err(err).Msgf("failed to diff account %x", []byte(owner))
			}

			logAccount(1)

			return nil
		})

		if foundAccountCountInRegistersV4 < registersV4.AccountCount() {

			log.Warn().Msgf("finding missing accounts that exist in v4, but are missing in v3, count: %v", registersV4.AccountCount()-foundAccountCountInRegistersV4)

			_ = registersV4.ForEachAccount(func(accountRegistersV4 *registers.AccountRegisters) error {
				owner := accountRegistersV4.Owner()
				if !registersV3.HasAccountOwner(owner) {
					rw.Write(accountMissing{
						Owner: owner,
						State: int(oldState),
					})
				}
				return nil
			})
		}

		return nil
	}

	type job struct {
		owner              string
		accountRegistersV3 *registers.AccountRegisters
		accountRegistersV4 *registers.AccountRegisters
	}

	type result struct {
		owner string
		err   error
	}

	jobs := make(chan job, nWorkers)

	results := make(chan result, nWorkers)

	g, ctx := errgroup.WithContext(context.Background())

	// Launch goroutines to diff accounts
	for i := 0; i < nWorkers; i++ {
		g.Go(func() (err error) {
			for job := range jobs {
				err := diffAccount(
					job.owner,
					job.accountRegistersV3,
					job.accountRegistersV4,
					chainID,
					rw,
				)

				select {
				case results <- result{owner: job.owner, err: err}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	// Launch goroutine to wait for workers and close result channel
	go func() {
		_ = g.Wait()
		close(results)
	}()

	// Launch goroutine to send account registers to jobs channel
	go func() {
		defer close(jobs)

		foundAccountCountInRegistersV4 := 0

		_ = registersV3.ForEachAccount(func(accountRegistersV3 *registers.AccountRegisters) (err error) {
			owner := accountRegistersV3.Owner()
			if !registersV4.HasAccountOwner(owner) {
				rw.Write(accountMissing{
					Owner: owner,
					State: int(newState),
				})

				return nil
			}

			foundAccountCountInRegistersV4++

			accountRegistersV4 := registersV4.AccountRegisters(owner)

			jobs <- job{
				owner:              owner,
				accountRegistersV3: accountRegistersV3,
				accountRegistersV4: accountRegistersV4,
			}

			return nil
		})

		if foundAccountCountInRegistersV4 < registersV4.AccountCount() {

			log.Warn().Msgf("finding missing accounts that exist in v4, but are missing in v3, count: %v", registersV4.AccountCount()-foundAccountCountInRegistersV4)

			_ = registersV4.ForEachAccount(func(accountRegistersV4 *registers.AccountRegisters) (err error) {
				owner := accountRegistersV4.Owner()
				if !registersV3.HasAccountOwner(owner) {
					rw.Write(accountMissing{
						Owner: owner,
						State: int(oldState),
					})
				}
				return nil
			})
		}
	}()

	// Gather results
	for result := range results {
		logAccount(1)
		if result.err != nil {
			log.Warn().Err(result.err).Msgf("failed to diff account %x", []byte(result.owner))
		}
	}

	if err := g.Wait(); err != nil {
		return err
	}

	log.Info().Msgf("Finished diffing accounts")

	return nil
}

func diffAccount(
	owner string,
	accountRegistersV3 *registers.AccountRegisters,
	accountRegistersV4 *registers.AccountRegisters,
	chainID flow.ChainID,
	rw reporters.ReportWriter,
) (err error) {
	address, err := common.BytesToAddress([]byte(owner))
	if err != nil {
		return err
	}

	migrations.NewAccountKeyDiffReporter(
		address,
		chainID,
		rw,
	).DiffKeys(
		accountRegistersV3,
		accountRegistersV4,
	)

	return nil
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
