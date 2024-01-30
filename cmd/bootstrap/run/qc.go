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

type ParticipantData struct {
	Participants []Participant
	Lookup       map[flow.Identifier]flow.DKGParticipant
	GroupKey     crypto.PublicKey
}

func (pd *ParticipantData) Identities() flow.IdentityList {
	nodes := make([]bootstrap.NodeInfo, 0, len(pd.Participants))
	for _, participant := range pd.Participants {
		nodes = append(nodes, participant.NodeInfo)
	}
	return bootstrap.ToIdentityList(nodes)
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
	committee, err := committees.NewStaticCommittee(identities, flow.Identifier{}, participantData.Lookup, participantData.GroupKey)
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

// GenerateQCParticipantData generates QC participant data used to create the
// random beacon and staking signatures on the QC.
//
// allNodes must be in the same order that was used when running the DKG.
func GenerateQCParticipantData(allNodes, internalNodes []bootstrap.NodeInfo, dkgData dkg.DKGData) (*ParticipantData, error) {

	// stakingNodes can include external validators, so it can be longer than internalNodes
	if len(allNodes) < len(internalNodes) {
		return nil, fmt.Errorf("need at least as many staking public keys as private keys (pub=%d, priv=%d)", len(allNodes), len(internalNodes))
	}

	// length of DKG participants needs to match stakingNodes, since we run DKG for external and internal validators
	if len(allNodes) != len(dkgData.PrivKeyShares) {
		return nil, fmt.Errorf("need exactly the same number of staking public keys as DKG private participants")
	}

	qcData := &ParticipantData{}

	participantLookup := make(map[flow.Identifier]flow.DKGParticipant)

	// the index here is important - we assume allNodes is in the same order as the DKG
	for i := 0; i < len(allNodes); i++ {
		// assign a node to a DGKdata entry, using the canonical ordering
		node := allNodes[i]
		participantLookup[node.NodeID] = flow.DKGParticipant{
			KeyShare: dkgData.PubKeyShares[i],
			Index:    uint(i),
		}
	}

	// the QC will be signed by everyone in internalNodes
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

	qcData.Lookup = participantLookup
	qcData.GroupKey = dkgData.PubGroupKey

	return qcData, nil
}
