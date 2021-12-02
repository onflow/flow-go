package run

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	hotstuffSig "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/crypto"
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
// NOTE: at the moment, we require private keys for one node because we we re-using the full business logic, which assumes that only consensus participants construct QCs, which also have produce votes.
// TODO: modularize QC construction code (and code to verify QC) to be instantiated without needing private keys.
func GenerateRootQC(block *flow.Block, votes []*model.Vote, participantData *ParticipantData, identities flow.IdentityList) (*flow.QuorumCertificate, error) {
	// create consensus committee's state
	committee, err := committees.NewStaticCommittee(identities, flow.Identifier{}, participantData.Lookup, participantData.GroupKey)
	if err != nil {
		return nil, err
	}

	hotstuffValidator, err := createValidator(committee)
	if err != nil {
		return nil, err
	}

	logger := zerolog.Logger{}
	var createdQC *flow.QuorumCertificate
	voteProcessorFactory := votecollector.NewBootstrapCombinedVoteProcessorFactory(logger, committee, func(qc *flow.QuorumCertificate) {
		createdQC = qc
	})
	processor, err := voteProcessorFactory.Create(model.ProposalFromFlow(block.Header, 0))
	if err != nil {
		return nil, err
	}
	hotBlock := model.GenesisBlockFromFlow(block.Header)

	// create the QC from the votes
	for _, vote := range votes {
		err := processor.Process(vote)
		if err != nil {
			return nil, err
		}
	}

	// validate QC
	err = hotstuffValidator.ValidateQC(createdQC, hotBlock)

	return createdQC, err
}

// GenerateRootBlockVotes generates votes for root block based on participantData
func GenerateRootBlockVotes(block *flow.Block, participantData *ParticipantData) ([]*model.Vote, error) {
	signers, err := createSigners(participantData, participantData.Identities())
	if err != nil {
		return nil, err
	}

	hotBlock := model.GenesisBlockFromFlow(block.Header)

	votes := make([]*model.Vote, 0, len(signers))
	for _, signer := range signers {
		vote, err := signer.CreateVote(hotBlock)
		if err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}
	return votes, nil
}

// createValidator creates validator that can validate votes and QC
func createValidator(committee hotstuff.Committee) (hotstuff.Validator, error) {
	packer := hotstuffSig.NewConsensusSigDataPacker(committee)
	verifier := verification.NewCombinedVerifier(committee, packer)

	forks := &mocks.ForksReader{}
	hotstuffValidator := validator.New(committee, forks, verifier)
	return hotstuffValidator, nil
}

// createSigners creates signers that can sign votes with both staking & random beacon keys
func createSigners(participantData *ParticipantData, identities flow.IdentityList) ([]hotstuff.Signer, error) {
	n := len(participantData.Participants)

	fmt.Println("len(participants)", len(participantData.Participants))
	fmt.Println("len(identities)", len(identities))
	for _, id := range identities {
		fmt.Println(id.NodeID, id.Address, id.StakingPubKey.String())
	}

	groupSize := uint(len(participantData.Participants))
	if groupSize < uint(n) {
		return nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v", groupSize, n)
	}

	signers := make([]hotstuff.Signer, n)
	for i, participant := range participantData.Participants {
		// get the participant private keys
		keys, err := participant.PrivateKeys()
		if err != nil {
			return nil, fmt.Errorf("could not get private keys for participant: %w", err)
		}

		me, err := local.New(participant.Identity(), keys.StakingKey)
		if err != nil {
			return nil, err
		}

		// create signer
		beaconStore := hotstuffSig.NewStaticRandomBeaconSignerStore(participant.RandomBeaconPrivKey)
		signer := verification.NewCombinedSigner(me, beaconStore)
		signers[i] = signer
	}

	return signers, nil
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

		if node.Stake == 0 {
			return nil, fmt.Errorf("node (id=%s) cannot have 0 stake", node.NodeID)
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
