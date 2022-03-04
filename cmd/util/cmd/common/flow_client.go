package common

import (
	"fmt"
	"strings"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/grpcutils"
)

const (
	DefaultAccessAPIPort       = "9000"
	DefaultAccessAPISecurePort = "9001"
)

type FlowClientConfig struct {
	AccessAddress    string
	AccessNodePubKey string
	Insecure         bool
}

func (f *FlowClientConfig) String() string {
	return fmt.Sprintf("AccessAddress: %s, AccessNodePubKey: %s, Insecure: %v", f.AccessAddress, f.AccessNodePubKey, f.Insecure)
}

// NewFlowClientConfig returns *FlowClientConfig
func NewFlowClientConfig(accessAddress, accessApiNodePubKey string, insecure bool) (*FlowClientConfig, error) {
	if accessAddress == "" {
		return nil, fmt.Errorf("failed to create  flow client connection option invalid access address: %s", accessAddress)
	}

	if !insecure {
		if accessApiNodePubKey == "" {
			return nil, fmt.Errorf("failed to create flow client connection option invalid access node networking public key: %s", accessApiNodePubKey)
		}
	}

	return &FlowClientConfig{accessAddress, accessApiNodePubKey, insecure}, nil
}

// FlowClient will return a secure or insecure flow client depending on *FlowClientConfig.Insecure
func FlowClient(conf *FlowClientConfig) (*client.Client, error) {
	if conf.Insecure {
		return insecureFlowClient(conf.AccessAddress)
	}

	return secureFlowClient(conf.AccessAddress, conf.AccessNodePubKey)
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
	flowClient, err := client.New(accessAddress, grpc.WithInsecure()) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("failed to create flow client %w", err)
	}

	return flowClient, nil
}

// FlowClientConfigs will assemble connection options for the flow client for each access node id
func FlowClientConfigs(accessNodeIDS []flow.Identifier, insecureAccessAPI bool, snapshot protocol.Snapshot) ([]*FlowClientConfig, error) {
	flowClientOpts := make([]*FlowClientConfig, 0)

	identities, err := snapshot.Identities(filter.HasNodeID(accessNodeIDS...))
	if err != nil {
		return nil, fmt.Errorf("failed get identities access node identities (ids=%v) from snapshot: %w", accessNodeIDS, err)
	}
	identities = identities.Sort(order.ByReferenceOrder(accessNodeIDS))

	// make sure we have identities for all the access node IDs provided
	if len(identities) != len(accessNodeIDS) {
		return nil, fmt.Errorf("failed to get %v distinct identities for all the access node IDs provided: %v, found %v", len(accessNodeIDS), accessNodeIDS, identities.NodeIDs())
	}

	// build a FlowClientConfig for each access node identity, these will be used to manage multiple flow client connections
	for i, identity := range identities {
		accessAddress := convertAccessAddrFromState(identity.Address, insecureAccessAPI)

		// remove the 0x prefix from network public keys
		networkingPubKey := strings.TrimPrefix(identity.NetworkPubKey.String(), "0x")

		opt, err := NewFlowClientConfig(accessAddress, networkingPubKey, insecureAccessAPI)
		if err != nil {
			return nil, fmt.Errorf("failed to get flow client connection option for access node ID (%d): %s %w", i, identity, err)
		}

		flowClientOpts = append(flowClientOpts, opt)
	}
	return flowClientOpts, nil
}

// convertAccessAddrFromState takes raw network address from the protocol state in the for of [DNS/IP]:PORT, removes the port and applies the appropriate
// port number depending on the insecureAccessAPI arg.
func convertAccessAddrFromState(address string, insecureAccessAPI bool) string {
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

// DefaultAccessNodeIDS will return all the access node IDS in the protocol state for staked access nodes
func DefaultAccessNodeIDS(snapshot protocol.Snapshot) ([]flow.Identifier, error) {
	identities, err := snapshot.Identities(filter.HasRole(flow.RoleAccess))
	if err != nil {
		return nil, fmt.Errorf("failed to get staked access node IDs from protocol state %w", err)
	}

	// sanity check something went wrong if no access node IDs are returned
	if len(identities) == 0 {
		return nil, fmt.Errorf("faled to get any access node IDs from protocol state")
	}

	return identities.NodeIDs(), nil
}

// FlowIDFromHexString convert flow node id(s) from hex string(s) to flow identifier(s)
func FlowIDFromHexString(accessNodeIDS ...string) ([]flow.Identifier, error) {
	// convert all IDS to flow.Identifier type, fail early if ID is invalid
	anIDS := make([]flow.Identifier, 0)
	for _, anID := range accessNodeIDS {
		id, err := flow.HexStringToIdentifier(anID)
		if err != nil {
			return nil, fmt.Errorf("could not get flow identifer from secured access node id (%s): %w", id, err)
		}

		anIDS = append(anIDS, id)
	}

	return anIDS, nil
}
