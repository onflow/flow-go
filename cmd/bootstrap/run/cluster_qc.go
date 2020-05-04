package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/signature"
	protoBadger "github.com/dapperlabs/flow-go/state/protocol/badger"
)

func GenerateClusterGenesisQC(participants []bootstrap.NodeInfo, block *flow.Block, clusterBlock *cluster.Block) (
	*model.QuorumCertificate, error) {

	ps, db, err := NewProtocolState(block)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	validators, signers, err := createClusterValidators(ps, participants, block)
	if err != nil {
		return nil, err
	}

	hotBlock := model.Block{
		BlockID:     clusterBlock.ID(),
		View:        clusterBlock.Header.View,
		ProposerID:  clusterBlock.Header.ProposerID,
		QC:          nil,
		PayloadHash: clusterBlock.Header.PayloadHash,
		Timestamp:   clusterBlock.Header.Timestamp,
	}

	votes := make([]*model.Vote, 0, len(signers))
	for _, signer := range signers {
		vote, err := signer.CreateVote(&hotBlock)
		if err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}

	// create the QC from the votes
	qc, err := signers[0].CreateQC(votes)
	if err != nil {
		return nil, err
	}

	// validate QC
	err = validators[0].ValidateQC(qc, &hotBlock)

	return qc, err
}

func createClusterValidators(ps *protoBadger.State, participants []bootstrap.NodeInfo, block *flow.Block) (
	[]hotstuff.Validator, []hotstuff.Signer, error) {
	n := len(participants)

	signers := make([]hotstuff.Signer, n)
	validators := make([]hotstuff.Validator, n)

	forks := &mocks.ForksReader{}

	nodeIDs := make([]flow.Identifier, 0, len(participants))
	for _, participant := range participants {
		nodeIDs = append(nodeIDs, participant.NodeID)
	}
	selector := filter.And(filter.HasNodeID(nodeIDs...), filter.HasStake(true))

	for i, participant := range participants {
		// get the participant keys
		keys, err := participant.PrivateKeys()
		if err != nil {
			return nil, nil, fmt.Errorf("could not retrieve private keys for participant: %w", err)
		}

		// create local module
		local, err := local.New(participant.Identity(), keys.StakingKey)
		if err != nil {
			return nil, nil, err
		}

		// create cluster committee state
		genesisBlockID := block.ID()
		blockTranslator := func(clusterBlock flow.Identifier) (flow.Identifier, error) { return genesisBlockID, nil }
		committee := committee.New(ps, blockTranslator, participant.NodeID, selector, nodeIDs)

		// create signer for participant
		provider := signature.NewAggregationProvider(encoding.CollectorVoteTag, local)
		signer := verification.NewSingleSigner(committee, provider, participant.NodeID)
		signers[i] = signer

		// create validator
		v := validator.New(committee, forks, signer)
		validators[i] = v
	}
	return validators, signers, nil
}
