package run

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
)

// GenerateClusterRootQC creates votes and generates a QC based on participant data
func GenerateClusterRootQC(participants []bootstrap.NodeInfo, clusterBlock *cluster.Block) (*flow.QuorumCertificate, error) {
	signers, err := createClusterSigners(participants)
	if err != nil {
		return nil, err
	}

	identities := bootstrap.ToIdentityList(participants)
	committee, err := committees.NewStaticCommittee(identities, flow.Identifier{}, nil, nil)
	if err != nil {
		return nil, err
	}

	hotstuffValidator, err := createClusterValidator(committee)
	if err != nil {
		return nil, fmt.Errorf("could not create cluster validator: %w", err)
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
			return nil, fmt.Errorf("could not create cluster vote: %w", err)
		}
		votes = append(votes, vote)
	}

	logger := zerolog.Logger{}
	var createdQC *flow.QuorumCertificate
	voteProcessorFactory := votecollector.NewBootstrapStakingVoteProcessorFactory(committee, func(qc *flow.QuorumCertificate) {
		createdQC = qc
	})
	processor, err := voteProcessorFactory.Create(logger, model.ProposalFromFlow(clusterBlock.Header, 0))
	if err != nil {
		return nil, fmt.Errorf("could not create cluster vote processor: %w", err)
	}

	// create the QC from the votes
	for _, vote := range votes {
		err := processor.Process(vote)
		if err != nil {
			return nil, fmt.Errorf("could not process vote: %w", err)
		}
	}

	if createdQC == nil {
		return nil, fmt.Errorf("not enough votes to create qc for bootstrapping")
	}

	// validate QC
	err = hotstuffValidator.ValidateQC(createdQC, &hotBlock)

	return createdQC, err
}

// createClusterValidator creates validator for cluster consensus
func createClusterValidator(committee hotstuff.Committee) (hotstuff.Validator, error) {
	verifier := verification.NewStakingVerifier()

	forks := &mocks.ForksReader{}
	hotstuffValidator := validator.New(committee, forks, verifier)
	return hotstuffValidator, nil
}

// createClusterSigners create signers for cluster signers
func createClusterSigners(participants []bootstrap.NodeInfo) ([]hotstuff.Signer, error) {
	n := len(participants)
	signers := make([]hotstuff.Signer, n)
	for i, participant := range participants {
		// get the participant keys
		keys, err := participant.PrivateKeys()
		if err != nil {
			return nil, fmt.Errorf("could not retrieve private keys for participant: %w", err)
		}

		// create local module
		me, err := local.New(participant.Identity(), keys.StakingKey)
		if err != nil {
			return nil, err
		}

		signer := verification.NewStakingSigner(me)
		signers[i] = signer
	}
	return signers, nil
}
