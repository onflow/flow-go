package run

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committee"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
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

func GenerateRootQC(block *flow.Block, participantData *ParticipantData) (*flow.QuorumCertificate, error) {

	validators, signers, err := createValidators(participantData)
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

	// manually aggregate sigs
	qc, err := signers[0].CreateQC(votes)
	if err != nil {
		return nil, err
	}

	// validate QC
	err = validators[0].ValidateQC(qc, hotBlock)

	return qc, err
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
		committee, err := committee.NewStaticCommittee(identities, local.NodeID(), participantData.Lookup, participantData.GroupKey)
		if err != nil {
			return nil, nil, err
		}

		// create signer
		stakingSigner := signature.NewAggregationProvider(encoding.ConsensusVoteTag, local)
		beaconSigner := signature.NewThresholdProvider(encoding.RandomBeaconTag, participant.RandomBeaconPrivKey)
		merger := signature.NewCombiner()
		signer := verification.NewCombinedSigner(committee, stakingSigner, beaconSigner, merger, participant.NodeID)
		signers[i] = signer

		// create validator
		v := validator.New(committee, forks, signer)
		validators[i] = v
	}

	return validators, signers, nil
}

func GenerateQCParticipantData(allNodes, internalNodes []bootstrap.NodeInfo, dkgData bootstrap.DKGData) (*ParticipantData, error) {

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

	// the QC will be signed by everyone in internalNodes
	for i, node := range internalNodes {
		// assign a node to a DGKdata entry, using the canonical ordering
		participantLookup[node.NodeID] = flow.DKGParticipant{
			KeyShare: dkgData.PubKeyShares[i],
			Index:    uint(i),
		}

		if node.NodeID == flow.ZeroID {
			return nil, fmt.Errorf("node id cannot be zero")
		}

		if node.Stake == 0 {
			return nil, fmt.Errorf("node (id=%s) cannot have 0 stake", node.NodeID)
		}

		qcData.Participants = append(qcData.Participants, Participant{
			NodeInfo:            node,
			RandomBeaconPrivKey: dkgData.PrivKeyShares[i],
		})
	}

	for i := len(internalNodes); i < len(allNodes); i++ {
		// assign a node to a DGKdata entry, using the canonical ordering
		node := allNodes[i]
		participantLookup[node.NodeID] = flow.DKGParticipant{
			KeyShare: dkgData.PubKeyShares[i],
			Index:    uint(i),
		}
	}

	qcData.Lookup = participantLookup
	qcData.GroupKey = dkgData.PubGroupKey

	return qcData, nil
}
