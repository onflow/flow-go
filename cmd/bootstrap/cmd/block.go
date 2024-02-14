package cmd

import (
	"encoding/hex"
	"time"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// constructRootHeader constructs a header for the root block.
func constructRootHeader(rootChain string, rootParent string, rootHeight uint64, rootTimestamp string) *flow.Header {
	chainID := parseChainID(rootChain)
	parentID := parseParentID(rootParent)
	height := rootHeight
	timestamp := parseRootTimestamp(rootTimestamp)

	return run.GenerateRootHeader(chainID, parentID, height, timestamp)
}

// constructRootBlock constructs a valid root block based on the given header, setup, and commit.
func constructRootBlock(rootHeader *flow.Header, setup *flow.EpochSetup, commit *flow.EpochCommit) *flow.Block {
	block := &flow.Block{
		Header:  rootHeader,
		Payload: nil,
	}
	block.SetPayload(flow.Payload{
		Guarantees:      nil,
		Seals:           nil,
		Receipts:        nil,
		Results:         nil,
		ProtocolStateID: inmem.ProtocolStateFromEpochServiceEvents(setup, commit).ID(),
	})
	return block
}

// constructRootEpochEvents constructs the epoch setup and commit events for the first epoch after spork.
func constructRootEpochEvents(
	firstView uint64,
	participants flow.IdentityList,
	assignments flow.AssignmentList,
	clusterQCs []*flow.QuorumCertificate,
	dkgData dkg.DKGData) (*flow.EpochSetup, *flow.EpochCommit) {
	epochSetup := &flow.EpochSetup{
		Counter:            flagEpochCounter,
		FirstView:          firstView,
		FinalView:          firstView + flagNumViewsInEpoch - 1,
		DKGPhase1FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase - 1,
		DKGPhase2FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*2 - 1,
		DKGPhase3FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*3 - 1,
		Participants:       participants.Sort(flow.Canonical[flow.Identity]).ToSkeleton(),
		Assignments:        assignments,
		RandomSource:       GenerateRandomSeed(flow.EpochSetupRandomSourceLength),
	}

	qcsWithSignerIDs := make([]*flow.QuorumCertificateWithSignerIDs, 0, len(clusterQCs))
	for i, clusterQC := range clusterQCs {
		members := assignments[i]
		signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(members, clusterQC.SignerIndices)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not decode signer IDs from clusterQC at index %v", i)
		}
		qcsWithSignerIDs = append(qcsWithSignerIDs, &flow.QuorumCertificateWithSignerIDs{
			View:      clusterQC.View,
			BlockID:   clusterQC.BlockID,
			SignerIDs: signerIDs,
			SigData:   clusterQC.SigData,
		})
	}

	epochCommit := &flow.EpochCommit{
		Counter:            flagEpochCounter,
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(qcsWithSignerIDs),
		DKGGroupKey:        dkgData.PubGroupKey,
		DKGParticipantKeys: dkgData.PubKeyShares,
	}
	return epochSetup, epochCommit
}

func parseChainID(chainID string) flow.ChainID {
	switch chainID {
	case "main":
		return flow.Mainnet
	case "test":
		return flow.Testnet
	case "preview":
		return flow.Previewnet
	case "sandbox":
		return flow.Sandboxnet
	case "bench":
		return flow.Benchnet
	case "local":
		return flow.Localnet
	default:
		log.Fatal().Str("chain_id", chainID).Msg("invalid chain ID")
		return ""
	}
}

func parseParentID(parentID string) flow.Identifier {
	decoded, err := hex.DecodeString(parentID)
	if err != nil {
		log.Fatal().Err(err).Str("parent_id", parentID).Msg("invalid parent ID")
	}
	var id flow.Identifier
	if len(decoded) != len(id) {
		log.Fatal().Str("parent_id", parentID).Msg("invalid parent ID length")
	}
	copy(id[:], decoded[:])
	return id
}

func parseRootTimestamp(timestamp string) time.Time {

	if timestamp == "" {
		return time.Now().UTC()
	}

	rootTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		log.Fatal().Str("root_time", timestamp).Msg("invalid root timestamp")
	}

	return rootTime
}
