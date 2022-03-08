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
func GenerateClusterRootQC(signers []bootstrap.NodeInfo, allCommitteeMembers flow.IdentityList, clusterBlock *cluster.Block) (*flow.QuorumCertificate, error) {
	clusterRootBlock := model.GenesisBlockFromFlow(clusterBlock.Header)

	// STEP 1: create votes for cluster root block
	votes, err := createRootBlockVotes(signers, clusterRootBlock)
	if err != nil {
		return nil, err
	}

	// STEP 2: create VoteProcessor
	committee, err := committees.NewStaticCommittee(allCommitteeMembers, flow.Identifier{}, nil, nil)
	if err != nil {
		return nil, err
	}
	var createdQC *flow.QuorumCertificate
	processor, err := votecollector.NewBootstrapStakingVoteProcessor(zerolog.Logger{}, committee, clusterRootBlock, func(qc *flow.QuorumCertificate) {
		createdQC = qc
	})
	if err != nil {
		return nil, fmt.Errorf("could not create cluster's StakingVoteProcessor: %w", err)
	}

	// STEP 3: feed the votes into the vote processor to create QC
	for _, vote := range votes {
		err := processor.Process(vote)
		if err != nil {
			return nil, fmt.Errorf("could not process vote: %w", err)
		}
	}
	if createdQC == nil {
		return nil, fmt.Errorf("not enough votes to create qc for bootstrapping")
	}

	// STEP 4: validate constructed QC
	val, err := createClusterValidator(committee)
	if err != nil {
		return nil, fmt.Errorf("could not create cluster validator: %w", err)
	}
	err = val.ValidateQC(createdQC, clusterRootBlock)

	return createdQC, err
}

// createClusterValidator creates validator for cluster consensus
func createClusterValidator(committee hotstuff.Committee) (hotstuff.Validator, error) {
	verifier := verification.NewStakingVerifier()

	forks := &mocks.ForksReader{}
	hotstuffValidator := validator.New(committee, forks, verifier)
	return hotstuffValidator, nil
}

// createRootBlockVotes generates a vote for the rootBlock from each participant
func createRootBlockVotes(participants []bootstrap.NodeInfo, rootBlock *model.Block) ([]*model.Vote, error) {
	votes := make([]*model.Vote, 0, len(participants))
	for _, participant := range participants {
		// create the participant's local identity
		keys, err := participant.PrivateKeys()
		if err != nil {
			return nil, fmt.Errorf("could not retrieve private keys for participant: %w", err)
		}
		me, err := local.New(participant.Identity(), keys.StakingKey)
		if err != nil {
			return nil, err
		}

		// generate root block vote
		vote, err := verification.NewStakingSigner(me).CreateVote(rootBlock)
		if err != nil {
			return nil, fmt.Errorf("could not create cluster vote for participant %v: %w", me.NodeID(), err)
		}
		votes = append(votes, vote)
	}
	return votes, nil
}
