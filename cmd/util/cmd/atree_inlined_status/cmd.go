package atree_inlined_status

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagOutputDirectory      string
	flagPayloads             string
	flagState                string
	flagStateCommitment      string
	flagNumOfPayloadToSample int
	flagNWorker              int
)

var Cmd = &cobra.Command{
	Use:   "atree-inlined-status",
	Short: "Check if atree payloads are inlined in given state",
	Run:   run,
}

const (
	ReporterName = "atree-inlined-status"

	numOfPayloadPerJob = 1_000
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
		8,
		"number of workers to use",
	)

	Cmd.Flags().IntVar(
		&flagNumOfPayloadToSample,
		"n-payloads",
		-1,
		"number of payloads to sample for inlined status (sample all payloads by default)",
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

	if flagNumOfPayloadToSample == 0 {
		log.Fatal().Msg("--n-payloads must be either > 0 or -1 (check all payloads)")
	}

	rw := reporters.NewReportFileWriterFactory(flagOutputDirectory, log.Logger).
		ReportWriter(ReporterName)
	defer rw.Close()

	var payloads []*ledger.Payload
	var err error

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

	totalPayloadCount := len(payloads)
	samplePayloadCount := len(payloads)

	if flagNumOfPayloadToSample > 0 && flagNumOfPayloadToSample < len(payloads) {
		samplePayloadCount = flagNumOfPayloadToSample
	}

	payloadsToSample := payloads

	if samplePayloadCount < totalPayloadCount {
		atreePayloadCount := 0
		i := 0
		for ; atreePayloadCount < samplePayloadCount; i++ {
			registerID, _, err := convert.PayloadToRegister(payloads[i])
			if err != nil {
				log.Fatal().Err(err).Msg("failed to convert payload to register")
			}

			if flow.IsSlabIndexKey(registerID.Key) {
				atreePayloadCount++
			}
		}

		payloadsToSample = payloads[:i]
	}

	atreeInlinedPayloadCount, atreeNonInlinedPayloadCount, err := checkAtreeInlinedStatus(payloadsToSample, flagNWorker)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to check atree inlined status")
	}

	rw.Write(stateStatus{
		InputPayloadFile:            flagPayloads,
		InputState:                  flagState,
		InputStateCommitment:        flagStateCommitment,
		TotalPayloadCount:           len(payloads),
		SamplePayloadCount:          len(payloadsToSample),
		AtreeInlinedPayloadCount:    atreeInlinedPayloadCount,
		AtreeNonInlinedPayloadCount: atreeNonInlinedPayloadCount,
	})
}

func checkAtreeInlinedStatus(payloads []*ledger.Payload, nWorkers int) (
	atreeInlinedPayloadCount int,
	atreeNonInlinedPayloadCount int,
	err error,
) {

	if len(payloads)/numOfPayloadPerJob < nWorkers {
		nWorkers = len(payloads) / numOfPayloadPerJob
	}

	log.Info().Msgf("checking atree payload inlined status...")

	if nWorkers <= 1 {
		// Skip goroutine to avoid overhead
		for _, p := range payloads {
			isAtreeSlab, isInlined, err := util.IsPayloadAtreeInlined(p)
			if err != nil {
				return 0, 0, err
			}

			if !isAtreeSlab {
				continue
			}

			if isInlined {
				atreeInlinedPayloadCount++
			} else {
				atreeNonInlinedPayloadCount++
			}
		}
		return
	}

	type job struct {
		payloads []*ledger.Payload
	}

	type result struct {
		atreeInlinedPayloadCount    int
		atreeNonInlinedPayloadCount int
	}

	numOfJobs := (len(payloads) + numOfPayloadPerJob - 1) / numOfPayloadPerJob

	jobs := make(chan job, numOfJobs)

	results := make(chan result, numOfJobs)

	g, ctx := errgroup.WithContext(context.Background())

	// Launch goroutine to check atree register inlined state
	for i := 0; i < nWorkers; i++ {
		g.Go(func() error {
			for job := range jobs {
				var result result

				for _, p := range job.payloads {
					isAtreeSlab, isInlined, err := util.IsPayloadAtreeInlined(p)
					if err != nil {
						return err
					}

					if !isAtreeSlab {
						continue
					}

					if isInlined {
						result.atreeInlinedPayloadCount++
					} else {
						result.atreeNonInlinedPayloadCount++
					}
				}

				select {
				case results <- result:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	// Launch goroutine to wait for workers and close output channel
	go func() {
		_ = g.Wait()
		close(results)
	}()

	// Send job to jobs channel
	payloadStartIndex := 0
	for {
		if payloadStartIndex == len(payloads) {
			close(jobs)
			break
		}

		endIndex := payloadStartIndex + numOfPayloadPerJob
		if endIndex > len(payloads) {
			endIndex = len(payloads)
		}

		jobs <- job{payloads: payloads[payloadStartIndex:endIndex]}

		payloadStartIndex = endIndex
	}

	// Gather results
	for result := range results {
		atreeInlinedPayloadCount += result.atreeInlinedPayloadCount
		atreeNonInlinedPayloadCount += result.atreeNonInlinedPayloadCount
	}

	log.Info().Msgf("waiting for goroutines...")

	if err := g.Wait(); err != nil {
		return 0, 0, err
	}

	return atreeInlinedPayloadCount, atreeNonInlinedPayloadCount, nil
}

type stateStatus struct {
	InputPayloadFile            string `json:",omitempty"`
	InputState                  string `json:",omitempty"`
	InputStateCommitment        string `json:",omitempty"`
	TotalPayloadCount           int
	SamplePayloadCount          int
	AtreeInlinedPayloadCount    int
	AtreeNonInlinedPayloadCount int
}
