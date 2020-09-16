package epochmgr

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	hotmodel "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// RootQCVoter is responsible for generating and submitting votes for the
// root quorum certificate of the upcoming epoch for this node's cluster.
type RootQCVoter struct {
	log    zerolog.Logger
	me     module.Local
	signer hotstuff.Signer
	state  protocol.State
	client module.QCContractClient // client to the QC aggregator smart contract
	epoch  protocol.Epoch          // the epoch for which we are generating root QC
}

// Vote handles the full procedure of generating a vote, submitting it to the
// epoch smart contract, and verifying submission. Returns an error only if
// there is a critical error that would make it impossible for the vote to be
// submitted. Otherwise, exits when the vote has been successfully submitted.
// It is safe to run multiple times per epoch.
func (voter *RootQCVoter) Vote(ctx context.Context) error {

	counter, err := voter.epoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get epoch counter: %w", err)
	}
	clusters, err := voter.epoch.Clustering()
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

		// check that our context is still valid
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// check whether we've already voted
		voted, err := voter.client.Voted(ctx)
		if err != nil {
			log.Error().Err(err).Msg("could not check vote status")
		} else if voted {
			log.Info().Msg("already voted - exiting QC vote process...")
			return nil
		}

		// submit the vote, this call will block until the transaction has
		// either succeeded or we are able to retry
		log.Info().Msg("submitting vote...")
		err = voter.client.SubmitVote(ctx, vote)
		if err != nil {
			log.Error().Err(err).Msg("could not submit vote")
		}

		// check that we're still in the setup phase, if we're not we can't
		// submit a vote anyway and must exit this process
		phase, err := voter.state.Final().Phase()
		if err != nil {
			log.Error().Err(err).Msg("could not get current phase")
			continue
		} else if phase != flow.EpochPhaseSetup {
			return fmt.Errorf("could not submit vote - no longer in setup phase")
		}
	}
}
