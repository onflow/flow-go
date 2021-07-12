package grpcutils

import (
	"crypto/tls"
	"fmt"

	libp2ptls "github.com/libp2p/go-libp2p-tls"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/network/p2p"
)

// DefaultMaxMsgSize use 16MB as the default message size limit.
// grpc library default is 4MB
const DefaultMaxMsgSize = 1024 * 1024 * 16

// X509Certificate generates a self-signed x509 TLS certificate from the given key. The generated certificate
// includes a libp2p extension that specifies the public key and the signature. The certificate does not include any
// SAN extension.
func X509Certificate(privKey crypto.PrivateKey) (*tls.Certificate, error) {

	// convert the Flow crypto private key to a Libp2p private crypto key
	libP2PKey, err := p2p.PrivKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("could not convert Flow key to libp2p key: %w", err)
	}

	// create a libp2p Identity from the libp2p private key
	id, err := libp2ptls.NewIdentity(libP2PKey)
	if err != nil {
		return nil, fmt.Errorf("could not generate identity: %w", err)
	}

	// extract the TLSConfig from it which will contains the generated x509 certificate
	libp2pTlsConfig, _ := id.ConfigForAny()

	// verify that exactly one certificate was generated for the given key
	if len(libp2pTlsConfig.Certificates) == 0 {
		return nil, fmt.Errorf("failed to generate the x509 certificate")
	}
	if len(libp2pTlsConfig.Certificates) > 1 {
		return nil, fmt.Errorf("unexpected number of x509 certificate chains generated")
	}

	return &libp2pTlsConfig.Certificates[0], nil
}

// DefaultServerTLSConfig returns the default TLS config with the given cert for a secure GRPC server
func DefaultServerTLSConfig(cert *tls.Certificate) *tls.Config {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ClientAuth:   tls.NoClientCert,
	}
	return tlsConfig
}
