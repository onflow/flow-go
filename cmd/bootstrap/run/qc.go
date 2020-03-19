package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/signature"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/test"
	"github.com/dapperlabs/flow-go/model/flow"
	hs "github.com/dapperlabs/flow-go/model/hotstuff"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dgraph-io/badger/v2"
)

type Signer struct {
	Identity             flow.Identity
	StakingPrivKey       crypto.PrivateKey
	RandomBeaconPrivKeys crypto.PrivateKey
}

type SignerData struct {
	DkgPubData *hotstuff.DKGPublicData
	Signers    []Signer
}

// TODO add a test with a validator as soon as https://github.com/dapperlabs/flow-go/blob/c34e266a3d8ec125a62361715db96cadd6747776/engine/consensus/hotstuff/validator_test.go#L186-L214 is merged
func GenerateGenesisQC(signerData SignerData, block flow.Block) (*hs.QuorumCertificate, error) {
	signers, err := createSigners(signerData)
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

	// TODO make dymanic
	// make votes
	vote1, err := signers[0].VoteFor(&hotBlock)
	if err != nil {
		return nil, err
	}
	vote2, err := signers[1].VoteFor(&hotBlock)
	if err != nil {
		return nil, err
	}
	vote3, err := signers[2].VoteFor(&hotBlock)
	if err != nil {
		return nil, err
	}

	// manually aggregate sigs
	aggsig, err := signers[0].Aggregate(&hotBlock,
		[]*hs.SingleSignature{vote1.Signature, vote2.Signature, vote3.Signature})
	if err != nil {
		return nil, err
	}

	// make QC
	qc := &hs.QuorumCertificate{
		View:                hotBlock.View,
		BlockID:             hotBlock.BlockID,
		AggregatedSignature: aggsig,
	}

	return qc, nil
}

func createSigners(signerData SignerData) ([]*signature.RandomBeaconAwareSigProvider, error) {
	if len(signerData.DkgPubData.IdToDKGParticipantMap) < len(signerData.Signers) {
		return nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v",
			len(signerData.DkgPubData.IdToDKGParticipantMap), len(signerData.Signers))
	}
	// TODO validate the individual dkg participants?

	ps, db, err := NewProtocolState()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	signers := make([]*signature.RandomBeaconAwareSigProvider, len(signerData.Signers))

	for i, signer := range signerData.Signers {
		// create signer
		signer, err := test.NewRandomBeaconSigProvider(ps, signerData.DkgPubData, &signer.Identity,
			signer.StakingPrivKey, signer.RandomBeaconPrivKeys)
		if err != nil {
			return nil, err
		}
		signers[i] = signer
	}
	return signers, nil
}

func NewProtocolState() (*protocol.State, *badger.DB, error) {
	dir, err := tempDBDir()
	if err != nil {
		return nil, nil, err
	}

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	if err != nil {
		return nil, nil, err
	}

	state, err := protocol.NewState(db)

	return state, db, err
}
