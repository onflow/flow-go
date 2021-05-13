package dkg

import (
	"crypto"
	"testing"

	"github.com/stretchr/testify/mock"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/engine/consensus/dkg"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	dkgmod "github.com/onflow/flow-go/module/dkg"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type nodeAccount struct {
	netID          *flow.Identity
	privKey        crypto.PrivateKey
	accountKey     *sdk.AccountKey
	accountID      string
	accountAddress sdk.Address
	accountSigner  sdkcrypto.Signer
	accountInfo    *bootstrap.NodeMachineAccountInfo
}

// node is an in-process node that only contains the engines relevant to DKG,
// ie. MessagingEngine and ReactorEngine
type node struct {
	testmock.GenericNode
	account           *nodeAccount
	dkgContractClient *dkgmod.Client
	keyStorage        storage.DKGKeys
	messagingEngine   *dkg.MessagingEngine
	reactorEngine     *dkg.ReactorEngine
}

func (n *node) Ready() {
	<-n.messagingEngine.Ready()
	<-n.reactorEngine.Ready()
}

func (n *node) Done() {
	<-n.messagingEngine.Done()
	<-n.reactorEngine.Done()
}

func (n *node) setEpochSetup(t *testing.T, epochSetup flow.EpochSetup, firstBlock *flow.Header) {
	// configure the state snapthost at firstBlock to return the desired
	// EpochSetup
	epoch := new(protocolmock.Epoch)
	epoch.On("Counter").Return(epochSetup.Counter, nil)
	epoch.On("InitialIdentities").Return(epochSetup.Participants, nil)
	epoch.On("DKGPhase1FinalView").Return(epochSetup.DKGPhase1FinalView, nil)
	epoch.On("DKGPhase2FinalView").Return(epochSetup.DKGPhase2FinalView, nil)
	epoch.On("DKGPhase3FinalView").Return(epochSetup.DKGPhase3FinalView, nil)
	epoch.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(epochSetup.RandomSource, nil)
	epochQuery := mocks.NewEpochQuery(t, epochSetup.Counter-1)
	epochQuery.Add(epoch)
	snapshot := new(protocolmock.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)
	state := new(protocolmock.MutableState)
	state.On("AtBlockID", firstBlock.ID()).Return(snapshot)
	n.GenericNode.State = state
	n.reactorEngine.State = state
}
