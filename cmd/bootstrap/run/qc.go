package run

import (
	"fmt"

	"github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	hotstuffSig "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
)

type Participant struct {
	bootstrap.NodeInfo
	RandomBeaconPrivKey crypto.PrivateKey
}

// ParticipantData represents a subset of all consensus participants that contributing to some signing process (at the moment, we only use
// it for the contributors for the root QC). For mainnet, this a *strict subset* of all consensus participants:
//   - In an early step during the bootstrapping process, every node operator locally generates votes for the root block from the nodes they
//     operate. During the vote-generation step, (see function `constructRootVotes`), `Participants` represents only the operator's own
//     nodes (generally a small minority of the entire consensus committee).
//   - During a subsequent step of the bootstrapping process, the Flow Foundation collects a supermajority of votes from the consensus
//     participants and constructs the root QC  (see function `constructRootQC`). Here, `Participants` is only populated with
//     the information of the internal nodes that the Flow Foundation runs, but not used.
//
// Furthermore, ParticipantData contains auxiliary data about the DKG to set up the random beacon. Note that the consensus committee 𝒞 and the
// DKG committee 𝒟 are generally _not_ identical. We explicitly want to support that _either_ set can have nodes that are not in
// the other set (formally 𝒟 \ 𝒞 ≠ ∅ and 𝒞 \ 𝒟 ≠ ∅). ParticipantData has no direct representation of the consensus committee 𝒞.
type ParticipantData struct {
	// Participants of the signing process: only members of the consensus committee can vote, i.e. contribute to the random
	// beacon (formally Participants ⊊ 𝒞). However, we allow for consensus committee members that are _not_ part of the DKG
	// committee 𝒟 (formally ∅ ≠ Participants \ 𝒟).
	Participants []Participant

	DKGCommittee map[flow.Identifier]flow.DKGParticipant // DKG committee 𝒟 (to set up the random beacon)
	DKGGroupKey  crypto.PublicKey                        // group key for the DKG committee 𝒟
}

func (pd *ParticipantData) Identities() flow.IdentityList {
	nodes := make([]bootstrap.NodeInfo, 0, len(pd.Participants))
	for _, participant := range pd.Participants {
		nodes = append(nodes, participant.NodeInfo)
	}
	return bootstrap.ToIdentityList(nodes)
}

// DKGData links nodes to their respective public DKG values. We cover the DKG committee 𝒟, meaning all nodes
// that were authorized to participate in the DKG (even if they did not participate or were unsuccessful).
// Specifically, the function returns:
//   - indexMap: a bijective mapping from the node IDs of the DKG committee to {0, 1, …, n-1},
//     with n = |𝒟| the size of the DKG committee 𝒟. In a nutshell, for a nodeID `d`, integer value
//     i := DKGIndexMap[d] denotes the index, by which the low-level cryptographic DKG protocol references
//     the participant. For details, please see the documentation of [flow.DKGIndexMap].
//   - keyShares: holds the public key share for every member of the DKG committee 𝒟 (irrespective
//     of successful participation). For a member of the DKG committee with nodeID `d`, the respective
//     public key share is keyShares[DKGIndexMap[d]].
//
// CAUTION: the returned DKG data may include identifiers for nodes which do not exist in the consensus committee
// and may NOT include entries for all nodes in the consensus committee.
func (pd *ParticipantData) DKGData() (indexMap flow.DKGIndexMap, keyShares []crypto.PublicKey) {
	indexMap = make(flow.DKGIndexMap, len(pd.DKGCommittee))
	keyShares = make([]crypto.PublicKey, len(pd.DKGCommittee))
	for nodeID, participant := range pd.DKGCommittee {
		indexMap[nodeID] = int(participant.Index)
		keyShares[participant.Index] = participant.KeyShare
	}
	return indexMap, keyShares
}

// GenerateRootQC generates QC for root block, caller needs to provide votes for root QC and
// participantData to build the QC.
// NOTE: at the moment, we require private keys for one node because we we re-using the full business logic,
// which assumes that only consensus participants construct QCs, which also have produce votes.
//
// TODO: modularize QC construction code (and code to verify QC) to be instantiated without needing private keys.
// It returns (qc, nil, nil) if a QC can be constructed with enough votes, and there is no invalid votes
// It returns (qc, invalidVotes, nil) if there are some invalid votes, but a QC can still be constructed
// It returns (nil, invalidVotes, err) if no qc can be constructed with not enough votes or running any any exception
func GenerateRootQC(block *flow.Block, votes []*model.Vote, participantData *ParticipantData, identities flow.IdentityList) (
	*flow.QuorumCertificate, // the constructed QC
	[]error, // return invalid votes error
	error, // exception or could not construct qc
) {
	// create consensus committee's state
	committee, err := committees.NewStaticCommittee(identities, flow.Identifier{}, participantData.DKGCommittee, participantData.DKGGroupKey)
	if err != nil {
		return nil, nil, err
	}

	// STEP 1: create VoteProcessor
	var createdQC *flow.QuorumCertificate
	hotBlock := model.GenesisBlockFromFlow(block.Header)
	processor, err := votecollector.NewBootstrapCombinedVoteProcessor(zerolog.Logger{}, committee, hotBlock, func(qc *flow.QuorumCertificate) {
		createdQC = qc
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not CombinedVoteProcessor processor: %w", err)
	}

	invalidVotes := make([]error, 0, len(votes))
	// STEP 2: feed the votes into the vote processor to create QC
	for _, vote := range votes {
		err := processor.Process(vote)

		// in case there are invalid votes, we continue process more votes,
		// so that finalizing block won't be interrupted by any invalid vote.
		// if no enough votes are collected, finalize will fail and exit anyway, because
		// no QC will be built.
		if err != nil {
			if model.IsInvalidVoteError(err) {
				invalidVotes = append(invalidVotes, err)
				continue
			}
			return nil, invalidVotes, fmt.Errorf("fail to process vote %v for block %v from signer %v: %w",
				vote.ID(),
				vote.BlockID,
				vote.SignerID,
				err)
		}
	}

	if createdQC == nil {
		return nil, invalidVotes, fmt.Errorf("QC is not created, total number of votes %v, expect to have 2/3 votes of %v participants",
			len(votes), len(identities))
	}

	// STEP 3: validate constructed QC
	val, err := createValidator(committee)
	if err != nil {
		return nil, invalidVotes, err
	}
	err = val.ValidateQC(createdQC)

	return createdQC, invalidVotes, err
}

// GenerateRootBlockVotes generates votes for root block based on participantData
func GenerateRootBlockVotes(block *flow.Block, participantData *ParticipantData) ([]*model.Vote, error) {
	hotBlock := model.GenesisBlockFromFlow(block.Header)
	n := len(participantData.Participants)
	fmt.Println("Number of staked consensus nodes: ", n)

	votes := make([]*model.Vote, 0, n)
	for _, p := range participantData.Participants {
		fmt.Println("generating votes from consensus participants: ", p.NodeID, p.Address, p.StakingPubKey().String())

		// create the participant's local identity
		keys, err := p.PrivateKeys()
		if err != nil {
			return nil, fmt.Errorf("could not get private keys for participant: %w", err)
		}
		me, err := local.New(p.Identity().IdentitySkeleton, keys.StakingKey)
		if err != nil {
			return nil, err
		}

		// create signer and use it to generate vote
		beaconStore := hotstuffSig.NewStaticRandomBeaconSignerStore(p.RandomBeaconPrivKey)
		vote, err := verification.NewCombinedSigner(me, beaconStore).CreateVote(hotBlock)
		if err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}
	return votes, nil
}

// createValidator creates validator that can validate votes and QC
func createValidator(committee hotstuff.DynamicCommittee) (hotstuff.Validator, error) {
	packer := hotstuffSig.NewConsensusSigDataPacker(committee)
	verifier := verification.NewCombinedVerifier(committee, packer)

	hotstuffValidator := validator.New(committee, verifier)
	return hotstuffValidator, nil
}

// GenerateQCParticipantData assembles the private information of a subset (`internalNodes`) of the consensus
// committee (`allNodes`).
//
// LIMITATION: this function only supports the 'trusted dealer' model, where for the consensus committee (`allNodes`)
// a trusted dealer generated the threshold-signature key (`dkgData` containing key shares and group key). Therefore,
// `allNodes` must be in the same order that was used when running the DKG.
func GenerateQCParticipantData(allNodes, internalNodes []bootstrap.NodeInfo, dkgData dkg.DKGData) (*ParticipantData, error) {
	// stakingNodes can include external validators, so it can be longer than internalNodes
	if len(allNodes) < len(internalNodes) {
		return nil, fmt.Errorf("need at least as many staking public keys as private keys (pub=%d, priv=%d)", len(allNodes), len(internalNodes))
	}

	// The following code is SHORTCUT assuming a trusted dealer at network genesis to initialize the Random Beacon
	// key shares for the first epoch. In addition to the staking keys (which are generated by each node individually),
	// the Random Beacon key shares are needed to sign the QC for the root block.
	// On the one hand, a trusted dealer can generate the key shares nearly instantaneously, which significantly
	// simplifies the coordination of the consensus committee prior to network genesis. On the other hand, there
	// are centralization concerns of employing a trusted dealer.
	// In the future, we want to re-use a DKG result from a prior Flow instance, instead of relying on a trusted dealer.
	// However, when using DKG results later (or re-using them to recover from Epoch Fallback Mode), there is a chance
	// that the DKG committee 𝒟 and the consensus committee 𝒞 might differ by some nodes. In this case, the following
	// logic would need to be generalized. Note that the output struct `ParticipantData`, which we construct below,
	// already handles cases with  𝒟 \ 𝒞 ≠ ∅ and/or 𝒞 \ 𝒟 ≠ ∅.
	//
	// However, the logic in this function only supports the trusted dealer model!
	// For further details see issue (Epic) https://github.com/onflow/flow-go/issues/6214
	if len(allNodes) != len(dkgData.PrivKeyShares) {
		return nil, fmt.Errorf("only trusted dealer for DKG supported: need exactly the same number of staking public keys as DKG private participants")
	}

	// the index here is important - we assume allNodes is in the same order as the DKG
	participantLookup := make(map[flow.Identifier]flow.DKGParticipant)
	for i, node := range allNodes {
		// assign a node to a DGKdata entry, using the canonical ordering
		participantLookup[node.NodeID] = flow.DKGParticipant{
			KeyShare: dkgData.PubKeyShares[i],
			Index:    uint(i),
		}
	}

	// the QC will be signed by everyone in internalNodes
	qcData := &ParticipantData{}
	for _, node := range internalNodes {
		if node.NodeID == flow.ZeroID {
			return nil, fmt.Errorf("node id cannot be zero")
		}
		if node.Weight == 0 {
			return nil, fmt.Errorf("node (id=%s) cannot have 0 weight", node.NodeID)
		}

		dkgParticipant, ok := participantLookup[node.NodeID]
		if !ok {
			return nil, fmt.Errorf("nonexistannt node id (%x) in participant lookup", node.NodeID)
		}
		dkgIndex := dkgParticipant.Index

		qcData.Participants = append(qcData.Participants, Participant{
			NodeInfo:            node,
			RandomBeaconPrivKey: dkgData.PrivKeyShares[dkgIndex],
		})
	}

	qcData.DKGCommittee = participantLookup
	qcData.DKGGroupKey = dkgData.PubGroupKey

	return qcData, nil
}
