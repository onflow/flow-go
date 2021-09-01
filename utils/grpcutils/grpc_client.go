package grpcutils

import (
	"encoding/hex"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go-sdk/crypto"
)

// SecureGRPCDialOpt creates a secure GRPC  dial option with TLS config
func SecureGRPCDialOpt(publicKeyHex string) (grpc.DialOption, error) {
	bytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode secured GRPC server public key hex %w", err)
	}

	publicFlowNetworkingKey, err := crypto.DecodePublicKey(crypto.ECDSA_P256, bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get public flow networking key could not decode public key bytes %w", err)
	}

	tlsConfig, err := DefaultClientTLSConfig(publicFlowNetworkingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get default TLS client config using public flow networking key %s %w", publicFlowNetworkingKey.String(), err)
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}
