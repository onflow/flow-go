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
	for i, id := range accessNodeIDS {
		nodeID, err := flow.HexStringToIdentifier(id)
		if err != nil {
			return nil, fmt.Errorf("could not get flow identifer from secured access node id: %s", id)
		}

		identities, err := snapshot.Identities(filter.HasNodeID(nodeID))
		if err != nil {
			return nil, fmt.Errorf("could not get identity of secure access node: %s", id)
		}

		if len(identities) < 1 {
			return nil, fmt.Errorf("could not find identity of secure access node: %s", id)
		}

		// remove gossip port from access address and add respective secure or insecure port
		var accessAddress strings.Builder
		accessAddress.WriteString(strings.Split(identities[0].Address, ":")[0])

		if insecureAccessAPI {
			accessAddress.WriteString(fmt.Sprintf(":%s", DefaultAccessAPIPort))
		} else {
			accessAddress.WriteString(fmt.Sprintf(":%s", DefaultAccessAPISecurePort))
		}

		// remove the 0x prefix from network public keys
		networkingPubKey := identities[0].NetworkPubKey.String()[2:]

		opt, err := NewFlowClientOpt(accessAddress.String(), networkingPubKey, insecureAccessAPI)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow client connection option for access node ID (%x): %s %w", i, id, err)
		}

		flowClientOpts = append(flowClientOpts, opt)
	}
	return flowClientOpts, nil
}
