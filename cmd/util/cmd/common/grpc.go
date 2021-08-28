package common

import (
	"fmt"

	"google.golang.org/grpc"

	grpcutils "github.com/onflow/flow-go/utils/grpc"
)

func GetGRPCDialOption(accessAddress, accessApiNodePubKey string, insecureAccessAPI bool) (grpc.DialOption, error) {
	if insecureAccessAPI {
		return grpc.WithInsecure(), nil
	}

	if accessAddress == "" {
		return nil, fmt.Errorf("invalid flag --access-address")
	}

	if accessApiNodePubKey == "" {
		return nil, fmt.Errorf("invalid flag --access-node-grpc-public-key")
	}

	dialOpts, err := grpcutils.SecureGRPCDialOpt(accessApiNodePubKey)
	if err != nil {
		return nil, fmt.Errorf("could get secured GRPC dial options %w", err)
	}

	return dialOpts, nil
}
