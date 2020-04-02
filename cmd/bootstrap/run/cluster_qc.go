package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/mock"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/protocol"
	protoBadger "github.com/dapperlabs/flow-go/protocol/badger"
)

func GenerateClusterGenesisQC(ccSigners []bootstrap.NodeInfo, block *flow.Block, clusterBlock *cluster.Block) (
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

	sigs := make([]*model.SingleSignature, len(signers))
	for i, signer := range signers {
		vote, err := signer.VoteFor(&hotBlock)
		if err != nil {
			return nil, err
		}
		sigs[i] = vote.Signature
	}

	// manually aggregate sigs
	// TODO replace this with proper aggregation as soon as the protocol state is fetched from a reference consensus
	// block
	aggsig, err := aggregateSigs(&hotBlock, sigs)
	if err != nil {
		return nil, err
	}

	// make QC
	qc := &model.QuorumCertificate{
		View:                hotBlock.View,
		BlockID:             hotBlock.BlockID,
		AggregatedSignature: aggsig,
	}

	// validate QC
	// The following check fails right now since the validation logic assumes that the block passed will also be used
	// for looking up the identities that signed it. That is not the case for collector blocks, they do not act as a
	// reference point for the protocol state right now.
	// TODO Uncomment as soon as the protocol state is fetched from a reference consensus block
	// err = validators[0].ValidateQC(qc, &hotBlock)

	return qc, err
}

func createClusterValidators(ps *protoBadger.State, ccSigners []ClusterSigner, block *flow.Block) (
	[]*validator.Validator, []*signature.StakingSigProvider, error) {
	n := len(ccSigners)

	signers := make([]*signature.StakingSigProvider, n)
	validators := make([]*validator.Validator, n)

	f := &mock.ForksReader{}

	for i, signer := range ccSigners {
		// create signer
		signerId := signer.Identity
		s, err := NewStakingProvider(ps, ccSigners, encoding.CollectorVoteTag, &signerId, signer.StakingPrivKey)
		if err != nil {
			return nil, nil, err
		}
		signers[i] = s

		// create view state
		vs, err := viewstate.New(ps, nil, signer.Identity.NodeID, filter.In(signersToIdentityList(ccSigners))) // TODO need to filter per cluster
		if err != nil {
			return nil, nil, err
		}

		// create validator
		v := validator.New(vs, f, s)
		validators[i] = v
	}
	return validators, signers, nil
}

// create a new StakingSigProvider
func NewStakingProvider(ps protocol.State, signers []ClusterSigner, tag string, id *flow.Identity, sk crypto.PrivateKey) (
	*signature.StakingSigProvider, error) {
	vs, err := viewstate.New(ps, nil, id.NodeID, filter.In(signersToIdentityList(signers))) // TODO need to filter per cluster
	if err != nil {
		return nil, fmt.Errorf("cannot create view state: %w", err)
	}
	me, err := local.New(id, sk)
	if err != nil {
		return nil, fmt.Errorf("cannot create local: %w", err)
	}

	sigProvider := signature.NewStakingSigProvider(vs, tag, me)
	return sigProvider, nil
}

// aggregateSigs function is a temporary workaround for the StakigSigner.Aggregate function. It is used here since the
// current implementation of StakigSigner.Aggregate assumes that the block that is signed is also the anchor for the
// identity list in the protocol state, which is not the case for cluster blocks.
// TODO: Replace this function ones the sig aggregation logic has been adjusted to refer to a consensus block with to
// get the identity list.
func aggregateSigs(block *model.Block, sigs []*model.SingleSignature) (*model.AggregatedSignature, error) {
	if len(sigs) == 0 { // ensure that sigs is not empty
		return nil, fmt.Errorf("cannot aggregate an empty slice of signatures")
	}

	aggStakingSig, signerIDs := unsafeAggregate(sigs) // unsafe aggregate staking sigs: crypto math only; will not catch error

	return &model.AggregatedSignature{
		StakingSignatures:     aggStakingSig,
		RandomBeaconSignature: nil,
		SignerIDs:             signerIDs,
	}, nil
}

func unsafeAggregate(sigs []*model.SingleSignature) ([]crypto.Signature, []flow.Identifier) {
	// This implementation is a naive way of aggregation the signatures. It will work, with
	// the downside of costing more bandwidth.
	// The more optimal way, which is the real aggregation, will be implemented when the crypto
	// API is available.
	aggStakingSig := make([]crypto.Signature, len(sigs))
	signerIDs := make([]flow.Identifier, len(sigs))
	for i, sig := range sigs {
		aggStakingSig[i] = sig.StakingSignature
		signerIDs[i] = sig.SignerID
	}
	return aggStakingSig, signerIDs
}

func signersToIdentityList(signers []ClusterSigner) flow.IdentityList {
	identities := make([]*flow.Identity, len(signers))
	for i, signer := range signers {
		identities[i] = &signer.Identity
	}
	return identities
}
