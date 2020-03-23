package run

import (
	"fmt"
	"io/ioutil"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	hs "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/protocol"
	protoBadger "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dgraph-io/badger/v2"
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

func GenerateGenesisQC(signerData SignerData, block *flow.Block) (*hs.QuorumCertificate, error) {
	ps, db, err := NewProtocolState()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	validators, signers, err := createValidators(ps, signerData, block)
	if err != nil {
		return nil, err
	}

	hotBlock := hs.Block{
		BlockID:     block.ID(),
		View:        block.View,
		ProposerID:  block.ProposerID,
		QC:          nil,
		PayloadHash: block.PayloadHash,
		Timestamp:   block.Timestamp,
	}

	sigs := make([]*hs.SingleSignature, len(signers))
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
	qc := &hs.QuorumCertificate{
		View:                hotBlock.View,
		BlockID:             hotBlock.BlockID,
		AggregatedSignature: aggsig,
	}

	// validate QC
	err = validators[0].ValidateQC(qc, &hotBlock)

	return qc, nil
}

func createValidators(ps *protoBadger.State, signerData SignerData, block *flow.Block) ([]*hotstuff.Validator, []*signature.RandomBeaconAwareSigProvider, error) {
	n := len(signerData.Signers)

	if len(signerData.DkgPubData.IdToDKGParticipantMap) < n {
		return nil, nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v",
			len(signerData.DkgPubData.IdToDKGParticipantMap), n)
	}

	err := ps.Mutate().Bootstrap(block)
	if err != nil {
		return nil, nil, err
	}

	signers := make([]*signature.RandomBeaconAwareSigProvider, n)
	viewStates := make([]*hotstuff.ViewState, n)
	validators := make([]*hotstuff.Validator, n)

	f := &mocks.ForksReader{}

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
		vs, err := hotstuff.NewViewState(ps, signerData.DkgPubData, signer.Identity.NodeID, filter.HasRole(flow.RoleConsensus))
		if err != nil {
			return nil, nil, err
		}
		viewStates[i] = vs

		// create validator
		v := hotstuff.NewValidator(vs, f, s)
		validators[i] = v
	}
	return validators, signers, nil
}

func NewProtocolState() (*protoBadger.State, *badger.DB, error) {
	dir, err := tempDBDir()
	if err != nil {
		return nil, nil, err
	}

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	if err != nil {
		return nil, nil, err
	}

	state, err := protoBadger.NewState(db)

	return state, db, err
}

// create a new RandomBeaconAwareSigProvider
func NewRandomBeaconSigProvider(ps protocol.State, dkgPubData *hotstuff.DKGPublicData, id *flow.Identity,
	stakingKey crypto.PrivateKey, randomBeaconKey crypto.PrivateKey) (*signature.RandomBeaconAwareSigProvider, error) {
	vs, err := hotstuff.NewViewState(ps, dkgPubData, id.NodeID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot create view state: %w", err)
	}
	me, err := local.New(id, stakingKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create local: %w", err)
	}

	sigProvider := signature.NewRandomBeaconAwareSigProvider(vs, me, randomBeaconKey)
	return &sigProvider, nil
}

func tempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-bootstrap-db")
}
