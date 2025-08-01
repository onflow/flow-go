package cmd

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// constructRootHeaderBody constructs a header body for the root block.
func constructRootHeaderBody(rootChain string, rootParent string, rootHeight uint64, rootView uint64, rootTimestamp string) (*flow.HeaderBody, error) {
	chainID := parseChainID(rootChain)
	parentID := parseParentID(rootParent)
	timestamp := parseRootTimestamp(rootTimestamp)

	return run.GenerateRootHeaderBody(chainID, parentID, rootHeight, rootView, timestamp)
}

// constructRootBlock constructs a valid root block based on the given header and protocol state ID for that block.
func constructRootBlock(rootHeaderBody *flow.HeaderBody, protocolStateID flow.Identifier) (*flow.Block, error) {
	payload, err := flow.NewPayload(
		flow.UntrustedPayload{
			Guarantees:      nil,
			Seals:           nil,
			Receipts:        nil,
			Results:         nil,
			ProtocolStateID: protocolStateID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct payload: %w", err)
	}

	block, err := flow.NewRootBlock(
		flow.UntrustedBlock{
			HeaderBody: *rootHeaderBody,
			Payload:    *payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct root block: %w", err)
	}

	return block, nil
}

// constructRootEpochEvents constructs the epoch setup and commit events for the first epoch after spork.
func constructRootEpochEvents(
	firstView uint64,
	participants flow.IdentityList,
	assignments flow.AssignmentList,
	clusterQCs []*flow.QuorumCertificate,
	dkgData dkg.ThresholdKeySet,
	dkgIndexMap flow.DKGIndexMap,
) (*flow.EpochSetup, *flow.EpochCommit, error) {
	epochSetup, err := flow.NewEpochSetup(
		flow.UntrustedEpochSetup{
			Counter:            flagEpochCounter,
			FirstView:          firstView,
			DKGPhase1FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase - 1,
			DKGPhase2FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*2 - 1,
			DKGPhase3FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*3 - 1,
			FinalView:          firstView + flagNumViewsInEpoch - 1,
			Participants:       participants.Sort(flow.Canonical[flow.Identity]).ToSkeleton(),
			Assignments:        assignments,
			RandomSource:       GenerateRandomSeed(flow.EpochSetupRandomSourceLength),
			TargetDuration:     flagEpochTimingDuration,
			TargetEndTime:      rootEpochTargetEndTime(),
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not construct epoch setup: %w", err)
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

	epochCommit, err := flow.NewEpochCommit(
		flow.UntrustedEpochCommit{
			Counter:            flagEpochCounter,
			ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(qcsWithSignerIDs),
			DKGGroupKey:        dkgData.PubGroupKey,
			DKGParticipantKeys: dkgData.PubKeyShares,
			DKGIndexMap:        dkgIndexMap,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not construct epoch commit: %w", err)
	}

	return epochSetup, epochCommit, nil
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
