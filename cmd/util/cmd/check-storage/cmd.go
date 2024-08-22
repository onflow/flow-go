package check_storage

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	moduleUtil "github.com/onflow/flow-go/module/util"
)

var (
	flagPayloads        string
	flagState           string
	flagStateCommitment string
	flagOutputDirectory string
	flagNWorker         int
)

var Cmd = &cobra.Command{
	Use:   "check-storage",
	Short: "Check storage health",
	Run:   run,
}

const (
	ReporterName = "storage-health"
)

func init() {

	Cmd.Flags().StringVar(
		&flagPayloads,
		"payloads",
		"",
		"Input payload file name",
	)

	Cmd.Flags().StringVar(
		&flagState,
		"state",
		"",
		"Input state file name",
	)

	Cmd.Flags().StringVar(
		&flagStateCommitment,
		"state-commitment",
		"",
		"Input state commitment",
	)

	Cmd.Flags().StringVar(
		&flagOutputDirectory,
		"output-directory",
		"",
		"Output directory",
	)

	_ = Cmd.MarkFlagRequired("output-directory")

	Cmd.Flags().IntVar(
		&flagNWorker,
		"n-workers",
		10,
		"number of workers to use",
	)
}

func run(*cobra.Command, []string) {

	if flagPayloads == "" && flagState == "" {
		log.Fatal().Msg("Either --payloads or --state must be provided")
	} else if flagPayloads != "" && flagState != "" {
		log.Fatal().Msg("Only one of --payloads or --state must be provided")
	}
	if flagState != "" && flagStateCommitment == "" {
		log.Fatal().Msg("--state-commitment must be provided when --state is provided")
	}

	// Create report in JSONL format
	rw := reporters.NewReportFileWriterFactoryWithFormat(flagOutputDirectory, log.Logger, reporters.ReportFormatJSONL).
		ReportWriter(ReporterName)
	defer rw.Close()

	var payloads []*ledger.Payload
	var err error

	// Read payloads from payload file or checkpoint file

	if flagPayloads != "" {
		log.Info().Msgf("Reading payloads from %s", flagPayloads)

		_, payloads, err = util.ReadPayloadFile(log.Logger, flagPayloads)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read payloads")
		}
	} else {
		log.Info().Msgf("Reading trie %s", flagStateCommitment)

		stateCommitment := util.ParseStateCommitment(flagStateCommitment)
		payloads, err = util.ReadTrie(flagState, stateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read state")
		}
	}

	log.Info().Msgf("Grouping %d payloads by accounts ...", len(payloads))

	// Group payloads by accounts

	payloadAccountGrouping := util.GroupPayloadsByAccount(log.Logger, payloads, flagNWorker)

	log.Info().Msgf(
		"Creating registers from grouped payloads (%d) ...",
		len(payloads),
	)

	registersByAccount, err := util.NewByAccountRegistersFromPayloadAccountGrouping(payloadAccountGrouping, flagNWorker)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create ByAccount registers from payload")
	}

	accountCount := registersByAccount.AccountCount()

	log.Info().Msgf(
		"Created registers from payloads (%d accounts, %d payloads)",
		accountCount,
		len(payloads),
	)

	failedAccountAddresses, err := checkStorageHealth(registersByAccount, flagNWorker, rw)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to check storage health")
	}

	if len(failedAccountAddresses) == 0 {
		log.Info().Msgf("All %d accounts are health", accountCount)
		return
	}

	log.Info().Msgf(
		"%d out of %d accounts reported storage health check issues.  See report %s for more details.",
		len(failedAccountAddresses),
		accountCount,
		ReporterName,
	)

	log.Info().Msgf("Accounts with storage health issues:")
	for _, address := range failedAccountAddresses {
		log.Info().Msgf("  %x", []byte(address))
	}
}

func checkStorageHealth(
	registersByAccount *registers.ByAccount,
	nWorkers int,
	rw reporters.ReportWriter,
) (failedAccountAddresses []string, err error) {

	accountCount := registersByAccount.AccountCount()

	nWorkers = min(accountCount, nWorkers)

	log.Info().Msgf("Checking storage health of %d accounts using %d workers ...", accountCount, nWorkers)

	logAccount := moduleUtil.LogProgress(
		log.Logger,
		moduleUtil.DefaultLogProgressConfig(
			"processing account group",
			accountCount,
		),
	)

	if nWorkers <= 1 {
		// Skip goroutine to avoid overhead
		err = registersByAccount.ForEachAccount(
			func(accountRegisters *registers.AccountRegisters) error {
				accountStorageIssues := checkAccountStorageHealth(accountRegisters, nWorkers)

				if len(accountStorageIssues) > 0 {
					failedAccountAddresses = append(failedAccountAddresses, accountRegisters.Owner())

					for _, issue := range accountStorageIssues {
						rw.Write(issue)
					}
				}

				logAccount(1)

				return nil
			})

		return failedAccountAddresses, err
	}

	type job struct {
		accountRegisters *registers.AccountRegisters
	}

	type result struct {
		owner  string
		issues []accountStorageIssue
	}

	jobs := make(chan job, nWorkers)

	results := make(chan result, nWorkers)

	g, ctx := errgroup.WithContext(context.Background())

	// Launch goroutine to check account storage health
	for i := 0; i < nWorkers; i++ {
		g.Go(func() error {
			for job := range jobs {
				issues := checkAccountStorageHealth(job.accountRegisters, nWorkers)

				result := result{
					owner:  job.accountRegisters.Owner(),
					issues: issues,
				}

				select {
				case results <- result:
				case <-ctx.Done():
					return ctx.Err()
				}

				logAccount(1)
			}
			return nil
		})
	}

	// Launch goroutine to wait for workers to finish and close results (output) channel
	go func() {
		defer close(results)
		err = g.Wait()
	}()

	// Launch goroutine to send job to jobs channel and close jobs (input) channel
	go func() {
		defer close(jobs)

		err = registersByAccount.ForEachAccount(
			func(accountRegisters *registers.AccountRegisters) error {
				jobs <- job{accountRegisters: accountRegisters}
				return nil
			})
		if err != nil {
			log.Err(err).Msgf("failed to iterate accounts by registersByAccount")
		}
	}()

	// Gather results
	for result := range results {
		if len(result.issues) > 0 {
			failedAccountAddresses = append(failedAccountAddresses, result.owner)
			for _, issue := range result.issues {
				rw.Write(issue)
			}
		}
	}

	return failedAccountAddresses, err
}

func checkAccountStorageHealth(accountRegisters *registers.AccountRegisters, nWorkers int) []accountStorageIssue {
	owner := accountRegisters.Owner()

	address, err := common.BytesToAddress([]byte(owner))
	if err != nil {
		return []accountStorageIssue{
			{
				Address: address.Hex(),
				Kind:    storageErrorKindString[otherErrorKind],
				Msg:     err.Error(),
			}}
	}

	var issues []accountStorageIssue

	// Check atree storage health

	ledger := &registers.ReadOnlyLedger{Registers: accountRegisters}
	storage := runtime.NewStorage(ledger, nil)

	err = util.CheckStorageHealth(address, storage, accountRegisters, migrations.AllStorageMapDomains, nWorkers)
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[atreeStorageErrorKind],
				Msg:     err.Error(),
			})
	}

	// TODO: check health of non-atree registers

	return issues
}

type storageErrorKind int

const (
	otherErrorKind storageErrorKind = iota
	atreeStorageErrorKind
)

var storageErrorKindString = map[storageErrorKind]string{
	otherErrorKind:        "error_check_storage_failed",
	atreeStorageErrorKind: "error_atree_storage",
}

type accountStorageIssue struct {
	Address string
	Kind    string
	Msg     string
}
