package run

import (
	"fmt"
	"io/ioutil"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mock"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/protocol"
	protoBadger "github.com/dapperlabs/flow-go/protocol/badger"
)

type Signer struct {
	Identity            flow.Identity
	StakingPrivKey      crypto.PrivateKey
	RandomBeaconPrivKey crypto.PrivateKey
}

type SignerData struct {
	DkgPubData *hotstuff.DKGPublicData
	Signers    []Signer
}

func GenerateGenesisQC(signerData SignerData, block *flow.Block) (*bootstrap.GenesisQC, error) {
	ps, db, err := NewProtocolState(block)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	validators, signers, err := createValidators(ps, signerData, block)
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

	sigs := make([]*model.SingleSignature, len(signers))
	for i, signer := range signers {
		vote, err := signer.VoteFor(&hotBlock)
		if err != nil {
			return nil, err
		}
		sigs[i] = vote.Signature
	}

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(&hotBlock, sigs)
	if err != nil {
		return nil, err
	}

	// make QC
	qc := model.QuorumCertificate{
		View:                hotBlock.View,
		BlockID:             hotBlock.BlockID,
		AggregatedSignature: aggsig,
	}

	// validate QC
	err = validators[0].ValidateQC(&qc, &hotBlock)

	return bootstrap.GenesisQC(qc), err
}

func createValidators(ps *protoBadger.State, signerData SignerData, block *flow.Block) ([]*validator.Validator, []*signature.RandomBeaconAwareSigProvider, error) {
	n := len(signerData.Signers)

	if len(signerData.DkgPubData.IdToDKGParticipantMap) < n {
		return nil, nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v",
			len(signerData.DkgPubData.IdToDKGParticipantMap), n)
	}

	signers := make([]*signature.RandomBeaconAwareSigProvider, n)
	validators := make([]*validator.Validator, n)

	f := &mock.ForksReader{}

	for i, signer := range signerData.Signers {
		// create signer
		signerId := signer.Identity
		s, err := NewRandomBeaconSigProvider(ps, signerData.DkgPubData, &signerId,
			signer.StakingPrivKey, signer.RandomBeaconPrivKey)
		if err != nil {
			return nil, nil, err
		}
		signers[i] = s

		// create view state
		vs, err := viewstate.New(ps, signerData.DkgPubData, signer.Identity.NodeID, filter.HasRole(flow.RoleConsensus))
		if err != nil {
			return nil, nil, err
		}

		// create validator
		v := validator.New(vs, f, s)
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

// create a new RandomBeaconAwareSigProvider
func NewRandomBeaconSigProvider(ps protocol.State, dkgPubData *hotstuff.DKGPublicData, id *flow.Identity,
	stakingKey crypto.PrivateKey, randomBeaconKey crypto.PrivateKey) (*signature.RandomBeaconAwareSigProvider, error) {
	vs, err := viewstate.New(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot create view state: %w", err)
	}
	me, err := local.New(id, stakingKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create local: %w", err)
	}

	sigProvider := signature.NewRandomBeaconAwareSigProvider(vs, me, randomBeaconKey)
	return sigProvider, nil
}

func tempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-bootstrap-db")
}
