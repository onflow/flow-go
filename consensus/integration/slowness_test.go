package integration_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/dgraph-io/badger"
)

type Headers struct {
	sync.Mutex
	headers map[flow.Identifier]*flow.Header
}

func (h *Headers) Store(header *flow.Header) error {
	h.Lock()
	defer h.Unlock()
	h.headers[header.ID()] = header
	return nil
}

func (h *Headers) ByBlockID(blockID flow.Identifier) (*flow.Header, error) {
	header, found := h.headers[blockID]
	if found {
		return header, nil
	}
	return nil, fmt.Errorf("can not find header by id: %v", blockID)
}

func (h *Headers) ByNumber(number uint64) (*flow.Header, error) {
	return nil, nil
}

type Views struct {
	sync.Mutex
	latest uint64
}

func (v *Views) StoreLatest(view uint64) error {
	v.Lock()
	defer v.Unlock()
	v.latest = view
	return nil
}

func (v *Views) RetrieveLatest() (uint64, error) {
	return v.latest, nil
}

type Builder struct {
	headers *Headers
}

func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header)) (*flow.Header, error) {
	headers := b.headers.headers
	parent, ok := headers[parentID]
	if !ok {
		return nil, fmt.Errorf("parent block not found (parent: %x)", parentID)
	}
	header := &flow.Header{
		ChainID:     "chain",
		ParentID:    parentID,
		Height:      parent.Height + 1,
		PayloadHash: unittest.IdentifierFixture(),
		Timestamp:   time.Now().UTC(),
	}
	setter(header)
	headers[header.ID()] = header
	return header, nil
}

type Signer struct {
	localID flow.Identifier
}

func (*Signer) CreateProposal(block *model.Block) *model.Proposal {
	proposal := &model.Proposal{
		Block:   block,
		SigData: nil,
	}
	return proposal
}
func (s *Signer) CreateVote(block *model.Block) *model.Vote {
	vote := &model.Vote{
		View:     block.View,
		BlockID:  block.BlockID,
		SignerID: s.localID,
		SigData:  nil,
	}
	return vote
}
func (*Signer) CreateQC(votes []*model.Vote) *model.QuorumCertificate {
	voterIDs := make([]flow.Identifier, 0, len(votes))
	for _, vote := range votes {
		voterIDs = append(voterIDs, vote.SignerID)
	}
	qc := &model.QuorumCertificate{
		View:      votes[0].View,
		BlockID:   votes[0].BlockID,
		SignerIDs: voterIDs,
		SigData:   nil,
	}
	return qc
}

func (*Signer) VerifyVote(voterID flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

func (*Signer) VerifyQC(voterIDs []flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

func createNode(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		// state, err := protocol.NewState(db)
		// require.NoError(t, err)
		// // generate random default identity
		// identity := unittest.IdentityFixture()
		//
		// index := uint(0)
		// localID := identity.ID()
		// zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
		// log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel).With().Timestamp().Uint("index", index).Hex("local_id", localID[:]).Logger()
		// notifier := notifications.NewLogConsumer(log)
		//
		// // initialize no-op metrics mock
		// metrics := &module.Metrics{}
		// metrics.On("HotStuffBusyDuration", mock.Anything)
		// metrics.On("HotStuffIdleDuration", mock.Anything)
		//
		// headers := &Headers{}
		// views := &Views{}
		//
		// // mocked dependencies
		// snapshot := &protocol.Snapshot{}
		// proto := &protocol.State{}
		//
		// participants := flow.IdentityList{identity}
		// // program the protocol snapshot behaviour
		// snapshot.On("Identities", mock.Anything).Return(
		// 	func(selector flow.IdentityFilter) flow.IdentityList {
		// 		return participants.Filter(selector)
		// 	},
		// 	nil,
		// )
		// for _, participant := range participants {
		// 	snapshot.On("Identity", participant.NodeID).Return(participant, nil)
		// }
		//
		// // make local
		// seed := make([]byte, crypto.KeyGenSeedMinLenBlsBls12381)
		// n, err := rand.Read(seed)
		// if err != nil {
		// 	return nil, err
		// }
		// if n < len(seed) {
		// 	return nil, fmt.Errorf("insufficient random bytes")
		// }
		// priv, err := crypto.GeneratePrivateKey(crypto.BlsBls12381, seed)
		// if err != nil {
		// 	return nil, err
		// }
		// local, err := local.New(nil, priv)
		// if err != nil {
		// 	return nil, err
		// }
		//
		// builder := &Builder{headers}
		//
		// // make network
		// network := nil
		//
		// payloadsDB := nil
		// blocksDB := nil
		//
		// signer := &Signer{identity.ID()}
		//
		// rootHeader := gensisHeader()
		// rootQC := gensisQC(rootHeader)
		// selector := filter.Any
		//
		// hot, err := consensus.NewParticipant(log, notifier, metrics, headers,
		// 	views, proto, local, builder, updater, signer, communicator, selector, rootHeader,
		// 	rootQC)
		//
		// if err != nil {
		// 	return nil, err
		// }
		//
		// // initialize the pending blocks cache
		// cache := buffer.NewPendingBlocks()
		//
		// // initialize the compliance engine
		// comp, err := compliance.New(log, network, local, proto, headers, payloadsDB, cache)
		// if err != nil {
		// 	return nil, fmt.Errorf("could not initialize compliance engine: %w", err)
		// }
		//
		// // initialize the synchronization engine
		// sync, err := synchronization.New(log, network, local, proto, blocksDB, comp)
		// if err != nil {
		// 	return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
		// }
		//
		// comp = comp.WithSynchronization(sync).WithConsensus(hot)
		//
	})
}
func Test3Nodes(t *testing.T) {
	// ccl1, err := createNode()
	// require.NoError(t, err)
	// ccl2, err := createNode()
	// require.NoError(t, err)
	// require.NotNil(t, ccl1)
	// require.NotNil(t, ccl2)
}
