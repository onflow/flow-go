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
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/signature"
)

func GenerateClusterRootQC(participants []bootstrap.NodeInfo, clusterBlock *cluster.Block) (*model.QuorumCertificate, error) {

	validators, signers, err := createClusterValidators(participants)
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

func createClusterValidators(participants []bootstrap.NodeInfo) ([]hotstuff.Validator, []hotstuff.Signer, error) {

	n := len(participants)
	identities := bootstrap.ToIdentityList(participants)

	signers := make([]hotstuff.Signer, n)
	validators := make([]hotstuff.Validator, n)

	forks := &mocks.ForksReader{}

	for i, participant := range participants {
		// get the participant keys
		keys, err := participant.PrivateKeys()
		if err != nil {
			return nil, nil, fmt.Errorf("could not retrieve private keys for participant: %w", err)
		}

		// create local module
		me, err := local.New(participant.Identity(), keys.StakingKey)
		if err != nil {
			return nil, nil, err
		}

		committee, err := committee.NewStaticCommittee(identities, me.NodeID(), nil, nil)
		if err != nil {
			return nil, nil, err
		}

		// create signer for participant
		provider := signature.NewAggregationProvider(encoding.CollectorVoteTag, me)
		signer := verification.NewSingleSigner(committee, provider, participant.NodeID)
		signers[i] = signer

		// create validator
		v := validator.New(committee, forks, signer)
		validators[i] = v
	}
	return validators, signers, nil
}
