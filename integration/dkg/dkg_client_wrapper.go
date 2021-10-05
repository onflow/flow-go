package dkg

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	model "github.com/onflow/flow-go/model/messages"
	dkgmod "github.com/onflow/flow-go/module/dkg"
)

var errClientDisabled = fmt.Errorf("dkg client artifically disabled for tests")

// DKGClientWrapper implements the DKGContractClient interface , and provides a
// mechanism to simulate a situations where the DKG node cannot access the DKG
// smart-contract. The client can be disabled so that all requests to the
// emulator return an error.
type DKGClientWrapper struct {
	client  *dkgmod.Client
	enabled bool
}

// NewDKGClientWrapper instantiates a new DKGClientWrapper
func NewDKGClientWrapper(client *dkgmod.Client) *DKGClientWrapper {
	return &DKGClientWrapper{
		client:  client,
		enabled: true,
	}
}

// Enable allows the client to call the DKG smart-contract
func (c *DKGClientWrapper) Enable() {
	c.enabled = true
}

// Disable short-circuits all calls to the DKG smart-contract
func (c *DKGClientWrapper) Disable() {
	c.enabled = false
}

// GetAccount implements the BaseClient interface
func (c *DKGClientWrapper) GetAccount(ctx context.Context) (*sdk.Account, error) {
	return c.client.GetAccount(ctx)
}

// SendTransaction implements the BaseClient interface
func (c *DKGClientWrapper) SendTransaction(ctx context.Context, tx *sdk.Transaction) (sdk.Identifier, error) {
	return c.client.SendTransaction(ctx, tx)
}

// WaitForSealed implements the BaseClient interface
func (c *DKGClientWrapper) WaitForSealed(ctx context.Context, txID sdk.Identifier, started time.Time) error {
	return c.client.WaitForSealed(ctx, txID, started)
}

// Broadcast implements the DKGContractClient interface
func (c *DKGClientWrapper) Broadcast(msg model.BroadcastDKGMessage) error {
	if !c.enabled {
		return fmt.Errorf("failed to broadcast DKG message: %w", errClientDisabled)
	}
	return c.client.Broadcast(msg)
}

// ReadBroadcast implements the DKGContractClient interface
func (c *DKGClientWrapper) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]messages.BroadcastDKGMessage, error) {
	if !c.enabled {
		return []messages.BroadcastDKGMessage{}, fmt.Errorf("failed to read DKG broadcast messages: %w", errClientDisabled)
	}
	return c.client.ReadBroadcast(fromIndex, referenceBlock)
}

// SubmitResult implements the DKGContractClient interface
func (c *DKGClientWrapper) SubmitResult(groupPubKey crypto.PublicKey, pubKeys []crypto.PublicKey) error {
	if !c.enabled {
		return fmt.Errorf("failed to submit DKG result: %w", errClientDisabled)
	}
	return c.client.SubmitResult(groupPubKey, pubKeys)
}
