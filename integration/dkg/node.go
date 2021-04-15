package dkg

import (
	"crypto"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	dkgmod "github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/storage"
)

type nodeAccount struct {
	netID          flow.Identifier
	privKey        crypto.PrivateKey
	accountKey     *sdk.AccountKey
	accountAddress sdk.Address
	accountSigner  sdkcrypto.Signer
	accountInfo    *bootstrap.NodeMachineAccountInfo
}

// node is an in-process node that only contains the engines relevant to DKG,
// ie. MessagingEngine and ReactorEngine
type node struct {
	mock.GenericNode
	account           *nodeAccount
	dkgContractClient *dkgmod.Client
	keyStorage        storage.DKGKeys
	messagingEngine   *dkg.MessagingEngine
	reactorEngine     *dkg.ReactorEngine
}

func (n node) Ready() {
	<-n.messagingEngine.Ready()
	<-n.reactorEngine.Ready()
}

func (n node) Done() {
	<-n.messagingEngine.Done()
	<-n.reactorEngine.Done()
}
