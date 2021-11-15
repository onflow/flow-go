package epochs

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/module/retrymiddleware"

	"github.com/sethvargo/go-retry"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
)

const (
	// retryMilliseconds is the number of milliseconds to wait between retries
	retryMilliseconds = 1000 * time.Millisecond

	// percentage of jitter to add to QC contract requests
	retryJitter = 10

	// update qc contract client after 2 consecutive failures
	retryMaxConsecutiveFailures = 2
)

// RootQCVoter is responsible for generating and submitting votes for the
// root quorum certificate of the upcoming epoch for this node's cluster.
type RootQCVoter struct {
	log                       zerolog.Logger
	me                        module.Local
	signer                    hotstuff.Signer
	state                     protocol.State
	qcContractClients         []module.QCContractClient // priority ordered array of client to the QC aggregator smart contract
	lastSuccessfulClientIndex int                       // index of the contract client that was last successful during retries
	wait                      time.Duration             // how long to sleep in between vote attempts
}

// NewRootQCVoter returns a new root QC voter, configured for a particular epoch.
func NewRootQCVoter(
	log zerolog.Logger,
	me module.Local,
	signer hotstuff.Signer,
	state protocol.State,
	contractClients []module.QCContractClient,
) *RootQCVoter {

	voter := &RootQCVoter{
		log:               log.With().Str("module", "root_qc_voter").Logger(),
		me:                me,
		signer:            signer,
		state:             state,
		qcContractClients: contractClients,
		wait:              time.Second * 10,
	}
	return voter
}

// Vote handles the full procedure of generating a vote, submitting it to the
// epoch smart contract, and verifying submission. Returns an error only if
// there is a critical error that would make it impossible for the vote to be
// submitted. Otherwise, exits when the vote has been successfully submitted.
//
// It is safe to run multiple times within a single setup phase.
func (voter *RootQCVoter) Vote(ctx context.Context, epoch protocol.Epoch) error {

	counter, err := epoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get epoch counter: %w", err)
	}
	clusters, err := epoch.Clustering()
	if err != nil {
		return fmt.Errorf("could not get clustering: %w", err)
	}
	cluster, clusterIndex, ok := clusters.ByNodeID(voter.me.NodeID())
	if !ok {
		return fmt.Errorf("could not find self in clustering")
	}

	log := voter.log.With().
		Uint64("epoch", counter).
		Uint("cluster_index", clusterIndex).
		Logger()

	log.Info().Msg("preparing to generate vote for cluster root qc")

	// create the canonical root block for our cluster
	root := clusterstate.CanonicalRootBlock(counter, cluster)
	// create a signable hotstuff model
	signable := hotmodel.GenesisBlockFromFlow(root.Header)

	vote, err := voter.signer.CreateVote(signable)
	if err != nil {
		return fmt.Errorf("could not create vote for cluster root qc: %w", err)
	}

	expRetry, err := retry.NewExponential(retryMilliseconds)
	if err != nil {
		log.Fatal().Err(err).Msg("create retry mechanism")
	}

	clientIndex, qcContractClient := voter.updateContractClient(voter.lastSuccessfulClientIndex, true)
	onMaxConsecutiveRetries := func(totalAttempts int) {
		clientIndex, qcContractClient = voter.updateContractClient(clientIndex, false)
		log.Warn().Msgf("retrying on attempt (%d) with fallback access node at index (%d)", totalAttempts, clientIndex)
	}
	afterConsecutiveRetries := retrymiddleware.AfterConsecutiveFailures(retryMaxConsecutiveFailures, retry.WithJitterPercent(retryJitter, expRetry), onMaxConsecutiveRetries)

	err = retry.Do(ctx, afterConsecutiveRetries, func(ctx context.Context) error {
		// check that we're still in the setup phase, if we're not we can't
		// submit a vote anyway and must exit this process
		phase, err := voter.state.Final().Phase()
		if err != nil {
			log.Error().Err(err).Msg("could not get current phase")
		} else if phase != flow.EpochPhaseSetup {
			return fmt.Errorf("could not submit vote - no longer in setup phase")
		}

		// check whether we've already voted, if we have we can exit early
		voted, err := qcContractClient.Voted(ctx)
		if err != nil {
			log.Error().Err(err).Msg("could not check vote status")
			return retry.RetryableError(err)
		} else if voted {
			log.Info().Msg("already voted - exiting QC vote process...")
			// update our last successful client index for future calls
			voter.lastSuccessfulClientIndex = clientIndex
			return nil
		}

		// submit the vote - this call will block until the transaction has
		// either succeeded or we are able to retry
		log.Info().Msg("submitting vote...")
		err = qcContractClient.SubmitVote(ctx, vote)
		if err != nil {
			log.Error().Err(err).Msg("could not submit vote - retrying...")
			return retry.RetryableError(err)
		}

		log.Info().Msg("successfully submitted vote - exiting QC vote process...")

		// update our last successful client index for future calls
		voter.lastSuccessfulClientIndex = clientIndex
		return nil
	})

	return err
}

// updateContractClient will return the last successful client index by default for all initial operations or else
// it will return the appropriate client index with respect to last successful and number of client.
func (voter *RootQCVoter) updateContractClient(clientIndex int, init bool) (int, module.QCContractClient) {
	// if initial call return lastSuccessfulClientIndex will either be the index of the last successful client or 0
	if init {
		return voter.lastSuccessfulClientIndex, voter.qcContractClients[voter.lastSuccessfulClientIndex]
	}

	if clientIndex == voter.lastSuccessfulClientIndex {
		if clientIndex == len(voter.qcContractClients)-1 {
			clientIndex = 0
		} else {
			clientIndex++
		}
	} else {
		clientIndex = voter.lastSuccessfulClientIndex
	}

	return clientIndex, voter.qcContractClients[clientIndex]
}
