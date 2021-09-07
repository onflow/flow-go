package epochs

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
)

// RootQCVoter is responsible for generating and submitting votes for the
// root quorum certificate of the upcoming epoch for this node's cluster.
type RootQCVoter struct {
	log    zerolog.Logger
	me     module.Local
	signer hotstuff.Signer
	state  protocol.State
	client module.QCContractClient // client to the QC aggregator smart contract

	wait time.Duration // how long to sleep in between vote attempts
}

// NewRootQCVoter returns a new root QC voter, configured for a particular epoch.
func NewRootQCVoter(
	log zerolog.Logger,
	me module.Local,
	signer hotstuff.Signer,
	state protocol.State,
	client module.QCContractClient,
) *RootQCVoter {

	voter := &RootQCVoter{
		log:    log.With().Str("module", "root_qc_voter").Logger(),
		me:     me,
		signer: signer,
		state:  state,
		client: client,
		wait:   time.Second * 10,
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

	attempts := 0
	for {
		attempts++
		log := log.With().Int("attempt", attempts).Logger()

		// for all attempts after the first, wait before re-trying
		if attempts > 1 {
			wait := voter.getWaitInterval(attempts - 2) // -2 so that we wait the base interval in the first Sleep
			log.Info().Msgf("waiting for %s before retry", wait.String())

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled: %w", ctx.Err())
			case <-time.After(wait):
				// proceed and re-submit vote
			}
		}

		// check that we're still in the setup phase, if we're not we can't
		// submit a vote anyway and must exit this process
		phase, err := voter.state.Final().Phase()
		if err != nil {
			log.Error().Err(err).Msg("could not get current phase")
		} else if phase != flow.EpochPhaseSetup {
			return fmt.Errorf("could not submit vote - no longer in setup phase")
		}

		// check whether we've already voted, if we have we can exit early
		voted, err := voter.client.Voted(ctx)
		if err != nil {
			log.Error().Err(err).Msg("could not check vote status")
			continue
		} else if voted {
			log.Info().Msg("already voted - exiting QC vote process...")
			return nil
		}

		// submit the vote - this call will block until the transaction has
		// either succeeded or we are able to retry
		log.Info().Msg("submitting vote...")
		err = voter.client.SubmitVote(ctx, vote)
		if err != nil {
			log.Error().Err(err).Msg("could not submit vote - retrying...")
			continue
		}

		log.Info().Msg("successfully submitted vote - exiting QC vote process...")
		return nil
	}
}

// getWaitInterval returns an interval to wait after the given number of attempts.
// The interval includes some jitter to avoid synchronization of requests from
// all collection nodes.
func (voter *RootQCVoter) getWaitInterval(attempts int) time.Duration {
	base := voter.wait << attempts                // base wait period on a geometric backoff
	jitter := float64(base) * rand.Float64() * .1 // add 10% jitter to avoid synchronization across cluster

	return time.Duration(base) + time.Duration(jitter)
}
