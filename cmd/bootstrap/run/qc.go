package run

import (
	"fmt"
	"io/ioutil"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/dkg"
	"github.com/dapperlabs/flow-go/state/protocol"
	protoBadger "github.com/dapperlabs/flow-go/state/protocol/badger"
)

type Participant struct {
	bootstrap.NodeInfo
	RandomBeaconPrivKey crypto.PrivateKey
}

type ParticipantData struct {
	DKGState     dkg.State
	Participants []Participant
}

func GenerateGenesisQC(participantData ParticipantData, block *flow.Block) (*model.QuorumCertificate, error) {
	ps, db, err := NewProtocolState(block)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	validators, signers, err := createValidators(ps, participantData, block)
	if err != nil {
		return nil, err
	}

	hotBlock := model.Block{
		BlockID:     block.ID(),
		View:        block.View,
		ProposerID:  block.ProposerID,
		QC:          nil,
		PayloadHash: block.PayloadHash,
		Timestamp:   block.Timestamp,
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

func createValidators(ps protocol.State, participantData ParticipantData, block *flow.Block) ([]hotstuff.Validator, []hotstuff.Signer, error) {
	n := len(participantData.Participants)

	groupSize, err := participantData.DKGState.GroupSize()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get DKG group size: %w", err)
	}
	if groupSize < uint(n) {
		return nil, nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v", groupSize, n)
	}

	signers := make([]hotstuff.Signer, n)
	validators := make([]hotstuff.Validator, n)

	forks := &mocks.ForksReader{}

	// create selector
	nodeIDs := make([]flow.Identifier, 0, len(participantData.Participants))
	for _, participant := range participantData.Participants {
		nodeIDs = append(nodeIDs, participant.NodeID)
	}
	selector := filter.HasNodeID(nodeIDs...)

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

		// create signer
		stakingSigner := signature.NewAggregationProvider(encoding.ConsensusVoteTag, local)
		beaconSigner := signature.NewThresholdProvider(encoding.RandomBeaconTag, participant.RandomBeaconPrivKey)
		merger := signature.NewCombiner()
		signer := verification.NewCombinedSigner(ps, participantData.DKGState, stakingSigner, beaconSigner, merger, selector, participant.NodeID)
		signers[i] = signer

		// create view state
		vs, err := committee.New(ps, participant.NodeID, selector)
		if err != nil {
			return nil, nil, err
		}

		// create validator
		v := validator.New(vs, forks, signer)
		validators[i] = v
	}

	return validators, signers, nil
}

func NewProtocolState(block *flow.Block) (*protoBadger.State, *badger.DB, error) {

	dir, err := tempDBDir()
	if err != nil {
		return nil, nil, err
	}

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	if err != nil {
		return nil, nil, err
	}

	state, err := protoBadger.NewState(db)
	if err != nil {
		return nil, nil, err
	}

	err = state.Mutate().Bootstrap(block)
	if err != nil {
		return nil, nil, err
	}

	return state, db, err
}

func tempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-bootstrap-db")
}
