package common

import (
	"fmt"
	"google.golang.org/grpc"
	"strings"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/grpcutils"
)

const (
	DefaultAccessNodeIDSMinimum = 2
	DefaultAccessAPIPort = "9000"
	DefaultAccessAPISecurePort = "9001"
)

type FlowClientOpt struct {
	AccessAddress    string
	AccessNodePubKey string
	Insecure         bool
}

func (f *FlowClientOpt) String() string {
	return fmt.Sprintf("AccessAddress: %s, AccessNodePubKey: %s, Insecure: %v", f.AccessAddress, f.AccessNodePubKey, f.Insecure)
}

// NewFlowClientOpt returns *FlowClientOpt
func NewFlowClientOpt(accessAddress, accessApiNodePubKey string, insecure bool) (*FlowClientOpt, error) {
	if accessAddress == "" {
		return nil, fmt.Errorf("failed to create  flow client connection option invalid access address: %s", accessAddress)
	}

	if !insecure {
		if accessApiNodePubKey == "" {
			return nil, fmt.Errorf("failed to create flow client connection option invalid access node networking public key: %s", accessApiNodePubKey)
		}
	}

	return &FlowClientOpt{accessAddress, accessApiNodePubKey, insecure}, nil
}

// FlowClient will return a secure or insecure flow client depending on *FlowClientOpt.Insecure
func FlowClient(opt *FlowClientOpt) (*client.Client, error) {
	if opt.Insecure {
		return insecureFlowClient(opt.AccessAddress)
	}

	return secureFlowClient(opt.AccessAddress, opt.AccessNodePubKey)
}

// secureFlowClient creates a flow client with secured GRPC connection
func secureFlowClient(accessAddress, accessApiNodePubKey string) (*client.Client, error) {
	dialOpts, err := grpcutils.SecureGRPCDialOpt(accessApiNodePubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create flow client with secured GRPC conn could get secured GRPC dial options %w", err)
	}

	// create flow client
	flowClient, err := client.New(accessAddress, dialOpts)
	if err != nil {
		return nil, err
	}

	return flowClient, nil
}

// insecureFlowClient creates flow client with insecure GRPC connection
func insecureFlowClient(accessAddress string) (*client.Client, error) {
	// create flow client
	flowClient, err := client.New(accessAddress, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create flow client %w", err)
	}

	return flowClient, nil
}

// PrepareFlowClientOpts will assemble connection options for the flow client for each access node id
func PrepareFlowClientOpts(accessNodeIDS []string, insecureAccessAPI bool, snapshot protocol.Snapshot) ([]*FlowClientOpt, error) {
	flowClientOpts := make([]*FlowClientOpt, 0)

	// convert all IDS to flow.Identifier type, fail early if ID is invalid
	anIDS := make([]flow.Identifier, 0)
	for _, anID := range accessNodeIDS {
		id, err := flow.HexStringToIdentifier(anID)
		if err != nil {
			return nil, fmt.Errorf("could not get flow identifer from secured access node id: %s", id)
		}

		anIDS = append(anIDS, id)
	}

	identities, err := snapshot.Identities(filter.HasNodeID(anIDS...))
	if err != nil {
		return nil, fmt.Errorf("failed get identities access node IDs from snapshot: %v", accessNodeIDS)
	}

	// make sure we have identities for all the access node IDs provided
	if len(identities) != len(accessNodeIDS) {
		return nil, fmt.Errorf("failed to get identity for all the access node IDs provided: %v, got %s", accessNodeIDS, identities)
	}

	// build a FlowClientOpt for each access node identity, these will be used to manage multiple flow client connections
	for i, identity := range identities {
		accessAddress := ConvertAccessAddrFromState(identity.Address, insecureAccessAPI)

		// remove the 0x prefix from network public keys
		networkingPubKey := identities[0].NetworkPubKey.String()[2:]

		opt, err := NewFlowClientOpt(accessAddress, networkingPubKey, insecureAccessAPI)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow client connection option for access node ID (%x): %s %w", i, identity, err)
		}

		flowClientOpts = append(flowClientOpts, opt)
	}
	return flowClientOpts, nil
}

// ConvertAccessAddrFromState takes raw network address from the protocol state in the for of [DNS/IP]:PORT, removes the port and applies the appropriate
// port number depending on the insecureAccessAPI arg.
func ConvertAccessAddrFromState(address string, insecureAccessAPI bool) string  {
	// remove gossip port from access address and add respective secure or insecure port
	var accessAddress strings.Builder
	accessAddress.WriteString(strings.Split(address, ":")[0])

	if insecureAccessAPI {
		accessAddress.WriteString(fmt.Sprintf(":%s", DefaultAccessAPIPort))
	} else {
		accessAddress.WriteString(fmt.Sprintf(":%s", DefaultAccessAPISecurePort))
	}

	return accessAddress.String()
}
