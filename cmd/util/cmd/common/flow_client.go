package common

import (
	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"
	grpcutils "github.com/onflow/flow-go/utils/grpc"

	"fmt"
)

// SecureFlowClient creates a flow client with secured GRPC connection
func SecureFlowClient(securedAccessAddress, accessApiNodePubKey string) (*client.Client, error) {
	if securedAccessAddress == "" {
		return nil, fmt.Errorf("failed to create  flow client with secured GRPC conn invalid flag --secure-access-address")
	}

	if accessApiNodePubKey == "" {
		return nil, fmt.Errorf("failed to create flow client with secured GRPC conn invalid flag --access-node-grpc-public-key")
	}

	dialOpts, err := grpcutils.SecureGRPCDialOpt(accessApiNodePubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create flow client with secured GRPC conn could get secured GRPC dial options %w", err)
	}

	// create flow client
	flowClient, err := client.New(securedAccessAddress, dialOpts)
	if err != nil {
		return nil, err
	}

	return flowClient, nil
}

// InsecureFlowClient creates flow client with insecure GRPC connection
func InsecureFlowClient(accessAddress string) (*client.Client, error) {
	if accessAddress == "" {
		return nil, fmt.Errorf("failed to create  flow client invalid flag --secure-access-address")
	}

	// create flow client
	flowClient, err := client.New(accessAddress, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create flow client %w", err)
	}

	return flowClient, nil
}
