package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/signature"
)

type Participant struct {
	bootstrap.NodeInfo
	RandomBeaconPrivKey crypto.PrivateKey
}

type ParticipantData struct {
	Participants []Participant
	Lookup       map[flow.Identifier]epoch.DKGParticipant
	GroupKey     crypto.PublicKey
}

func (pd *ParticipantData) Identities() flow.IdentityList {
	nodes := make([]bootstrap.NodeInfo, 0, len(pd.Participants))
	for _, participant := range pd.Participants {
		nodes = append(nodes, participant.NodeInfo)
	}
	return bootstrap.ToIdentityList(nodes)
}

func GenerateRootQC(block *flow.Block, participantData ParticipantData) (*model.QuorumCertificate, error) {

	validators, signers, err := createValidators(participantData)
	if err != nil {
		return nil, err
	}

	hotBlock := model.Block{
		BlockID:     block.ID(),
		View:        block.Header.View,
		ProposerID:  block.Header.ProposerID,
		QC:          nil,
		PayloadHash: block.Header.PayloadHash,
		Timestamp:   block.Header.Timestamp,
	}

	votes := make([]*model.Vote, 0, len(signers))
	for _, signer := range signers {
		vote, err := signer.CreateVote(&hotBlock)
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
	err = validators[0].ValidateQC(qc, &hotBlock)

	return qc, err
}

func createValidators(participantData ParticipantData) ([]hotstuff.Validator, []hotstuff.Signer, error) {
	n := len(participantData.Participants)
	identities := participantData.Identities()

	groupSize := uint(len(participantData.Participants))
	if groupSize < uint(n) {
		return nil, nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v", groupSize, n)
	}

	signers := make([]hotstuff.Signer, n)
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
