package run

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/signature"
	protoBadger "github.com/dapperlabs/flow-go/state/protocol/badger"
)

type ClusterSigner struct {
	Identity       *flow.Identity
	StakingPrivKey crypto.PrivateKey
}

func GenerateClusterGenesisQC(ccSigners []ClusterSigner, block *flow.Block, clusterBlock *cluster.Block) (
	*model.QuorumCertificate, error) {

	ps, db, err := NewProtocolState(block)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	_, signers, err := createClusterValidators(ps, ccSigners, block)
	if err != nil {
		return nil, err
	}

	hotBlock := model.Block{
		BlockID:     clusterBlock.ID(),
		View:        clusterBlock.View,
		ProposerID:  clusterBlock.ProposerID,
		QC:          nil,
		PayloadHash: clusterBlock.PayloadHash,
		Timestamp:   clusterBlock.Timestamp,
	}

	votes := make([]*model.Vote, len(signers))
	for i, signer := range signers {
		vote, err := signer.CreateVote(&hotBlock)
		if err != nil {
			return nil, err
		}
		votes[i] = vote
	}

	// make QC
	qc, err := signers[0].CreateQC(votes)
	if err != nil {
		return nil, err
	}

	// validate QC
	// The following check fails right now since the validation logic assumes that the block passed will also be used
	// for looking up the identities that signed it. That is not the case for collector blocks, they do not act as a
	// reference point for the protocol state right now.
	// TODO Uncomment as soon as the protocol state is fetched from a reference consensus block
	// err = validators[0].ValidateQC(qc, &hotBlock)

	return qc, err
}

func createClusterValidators(ps *protoBadger.State, ccParticipants []ClusterSigner, block *flow.Block) (
	[]hotstuff.Validator, []hotstuff.Signer, error) {
	n := len(ccParticipants)

	signers := make([]hotstuff.Signer, n)
	validators := make([]hotstuff.Validator, n)

	f := &mocks.ForksReader{}

	for i, participant := range ccParticipants {
		// create signer
		provider := signature.NewAggregationProvider(encoding.CollectorVoteTag, participant.StakingPrivKey)
		stakingSigner := verification.NewSingleSigner(ps, provider, filter.Any, participant.Identity.NodeID)
		signers[i] = stakingSigner

		// create view state
		vs, err := viewstate.New(ps, nil, participant.Identity.NodeID, filter.In(signersToIdentityList(ccParticipants))) // TODO need to filter per cluster
		if err != nil {
			return nil, nil, err
		}

		// create validator
		v := validator.New(vs, f, stakingSigner)
		validators[i] = v
	}
	return validators, signers, nil
}

func signersToIdentityList(signers []ClusterSigner) flow.IdentityList {
	identities := make([]*flow.Identity, len(signers))
	for i, signer := range signers {
		identities[i] = signer.Identity
	}
	return identities
}
