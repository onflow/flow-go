package fetcher

import (
	"fmt"
	"regexp"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
)

var onlyOnflowRegex = regexp.MustCompile(`.*\.onflow\.org:3569$`)

type CollectionFetcher struct {
	log     zerolog.Logger
	request module.Requester // used to request collections
	state   protocol.State
	// This is included to temporarily work around an issue observed on a small number of ENs.
	// It works around an issue where some collection nodes are not configured with enough
	// file descriptors causing connection failures.
	onflowOnlyLNs bool
}

func NewCollectionFetcher(
	log zerolog.Logger,
	request module.Requester,
	state protocol.State,
	onflowOnlyLNs bool,
) *CollectionFetcher {
	return &CollectionFetcher{
		log:           log.With().Str("component", "ingestion_engine_collection_fetcher").Logger(),
		request:       request,
		state:         state,
		onflowOnlyLNs: onflowOnlyLNs,
	}
}

// FetchCollection decides which collection nodes to fetch the collection from
// No error is expected during normal operation
func (e *CollectionFetcher) FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error {
	guarantors, err := protocol.FindGuarantors(e.state, guarantee)
	if err != nil {
		// execution node executes certified blocks, which means there is a quorum of consensus nodes who
		// have validated the block payload. And that validation includes checking the guarantors are correct.
		// Based on that assumption, failing to find guarantors for guarantees contained in an incorporated block
		// should be treated as fatal error
		e.log.Fatal().Err(err).Msgf("failed to find guarantors for guarantee %v at block %v, height %v",
			guarantee.ID(),
			blockID,
			height,
		)
		return fmt.Errorf("could not find guarantors: %w", err)
	}

	filters := []flow.IdentityFilter{
		filter.HasNodeID(guarantors...),
	}

	// This is included to temporarily work around an issue observed on a small number of ENs.
	// It works around an issue where some collection nodes are not configured with enough
	// file descriptors causing connection failures. This will be removed once a
	// proper fix is in place.
	if e.onflowOnlyLNs {
		// func(Identity("verification-049.mainnet20.nodes.onflow.org:3569")) => true
		// func(Identity("verification-049.hello.org:3569")) => false
		filters = append(filters, func(identity *flow.Identity) bool {
			return onlyOnflowRegex.MatchString(identity.Address)
		})
	}

	// queue the collection to be requested from one of the guarantors
	e.request.EntityByID(guarantee.ID(), filter.And(
		filters...,
	))

	return nil
}

func (e *CollectionFetcher) Force() {
	e.request.Force()
}
