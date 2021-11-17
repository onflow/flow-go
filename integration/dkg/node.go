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
	dkgContractClient *DKGClientWrapper
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

// setEpochs configures the mock state snapthost at firstBlock to return the
// desired current and next epochs
func (n *node) setEpochs(t *testing.T, currentSetup flow.EpochSetup, nextSetup flow.EpochSetup, firstBlock *flow.Header) {

	currentEpoch := new(protocolmock.Epoch)
	currentEpoch.On("Counter").Return(currentSetup.Counter, nil)
	currentEpoch.On("InitialIdentities").Return(currentSetup.Participants, nil)
	currentEpoch.On("DKGPhase1FinalView").Return(currentSetup.DKGPhase1FinalView, nil)
	currentEpoch.On("DKGPhase2FinalView").Return(currentSetup.DKGPhase2FinalView, nil)
	currentEpoch.On("DKGPhase3FinalView").Return(currentSetup.DKGPhase3FinalView, nil)
	currentEpoch.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(nextSetup.RandomSource, nil)

	nextEpoch := new(protocolmock.Epoch)
	nextEpoch.On("Counter").Return(nextSetup.Counter, nil)
	nextEpoch.On("InitialIdentities").Return(nextSetup.Participants, nil)
	nextEpoch.On("Seed", mock.Anything, mock.Anything, mock.Anything).Return(nextSetup.RandomSource, nil)

	epochQuery := mocks.NewEpochQuery(t, currentSetup.Counter)
	epochQuery.Add(currentEpoch)
	epochQuery.Add(nextEpoch)
	snapshot := new(protocolmock.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)
	snapshot.On("Phase").Return(flow.EpochPhaseStaking, nil)
	snapshot.On("Head").Return(firstBlock, nil)
	state := new(protocolmock.MutableState)
	state.On("AtBlockID", firstBlock.ID()).Return(snapshot)
	state.On("Final").Return(snapshot)
	n.GenericNode.State = state
	n.reactorEngine.State = state
}
