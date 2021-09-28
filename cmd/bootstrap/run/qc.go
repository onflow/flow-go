package run

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/signature"
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
// NOTE: only one DKG private key is required to sign the QC, for all others entries we can
// provide only identity info without private keys.
func GenerateRootQC(block *flow.Block, votes []*model.Vote, participantData *ParticipantData) (*flow.QuorumCertificate, error) {
	validators, signers, err := createValidators(participantData)
	if err != nil {
		return nil, err
	}

	hotBlock := model.GenesisBlockFromFlow(block.Header)

	// manually aggregate sigs
	qc, err := signers[0].CreateQC(votes)
	if err != nil {
		return nil, err
	}

	// validate QC
	err = validators[0].ValidateQC(qc, hotBlock)

	return qc, err
}

// GenerateRootBlockVotes generates votes for root block based on participantData
func GenerateRootBlockVotes(block *flow.Block, participantData *ParticipantData) ([]*model.Vote, error) {
	_, signers, err := createValidators(participantData)
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

func createValidators(participantData *ParticipantData) ([]hotstuff.Validator, []hotstuff.SignerVerifier, error) {
	n := len(participantData.Participants)
	identities := participantData.Identities()

	groupSize := uint(len(participantData.Participants))
	if groupSize < uint(n) {
		return nil, nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v", groupSize, n)
	}

	signers := make([]hotstuff.SignerVerifier, n)
	validators := make([]hotstuff.Validator, n)

	forks := &mocks.ForksReader{}

	for i, participant := range participantData.Participants {
		// get the participant private keys
		keys, err := participant.PrivateKeys()
		if err != nil {
			return nil, nil, fmt.Errorf("could not get private keys for participant: %w", err)
		}

		local, err := local.New(participant.Identity(), keys.StakingKey)
		if err != nil {
			return nil, nil, err
		}

		// create consensus committee's state
		committee, err := committees.NewStaticCommittee(identities, local.NodeID(), participantData.Lookup, participantData.GroupKey)
		if err != nil {
			return nil, nil, err
		}

		// create signer
		merger := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)
		stakingSigner := signature.NewAggregationProvider(encoding.ConsensusVoteTag, local)
		beaconVerifier := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
		beaconSigner := signature.NewThresholdProvider(encoding.RandomBeaconTag, participant.RandomBeaconPrivKey)
		beaconStore := signature.NewSingleSignerStore(beaconSigner)
		signer := verification.NewCombinedSigner(committee, stakingSigner, beaconVerifier, merger, beaconStore, participant.NodeID)
		signers[i] = signer

		// create validator
		v := validator.New(committee, forks, signer)
		validators[i] = v
	}

	return validators, signers, nil
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
