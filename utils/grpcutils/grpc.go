package grpcutils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"

	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
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
	libP2PKey, err := p2p.LibP2PPrivKeyFromFlow(privKey)
	if err != nil {
		return nil, fmt.Errorf("could not convert Flow key to libp2p key: %w", err)
	}

	// create a libp2p Identity from the libp2p private key
	id, err := libp2ptls.NewIdentity(libP2PKey)
	if err != nil {
		return nil, fmt.Errorf("could not generate identity: %w", err)
	}

	// extract the TLSConfig from it which will contain the generated x509 certificate
	// (ignore the public key that is returned - it is the public key of the private key used to generate the ID)
	libp2pTlsConfig, _ := id.ConfigForAny()

	// verify that exactly one certificate was generated for the given key
	certCount := len(libp2pTlsConfig.Certificates)
	if certCount != 1 {
		return nil, fmt.Errorf("invalid count for the generated x509 certificate: %d", certCount)
	}

	return &libp2pTlsConfig.Certificates[0], nil
}

// DefaultServerTLSConfig returns the default TLS server config with the given cert for a secure GRPC server
func DefaultServerTLSConfig(cert *tls.Certificate) *tls.Config {
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{*cert},
		ClientAuth:   tls.NoClientCert,
	}
	return tlsConfig
}

// ServerAuthError is an error returned when the server authentication fails
type ServerAuthError struct {
	message string
}

// newServerAuthError constructs a new ServerAuthError
func newServerAuthError(msg string, args ...interface{}) *ServerAuthError {
	return &ServerAuthError{message: fmt.Sprintf(msg, args...)}
}

func (e ServerAuthError) Error() string {
	return e.message
}

// IsServerAuthError checks if the input error is of a ServerAuthError type
func IsServerAuthError(err error) bool {
	_, ok := err.(*ServerAuthError)
	return ok
}

// DefaultClientTLSConfig returns the default TLS client config with the given public key for a secure GRPC client
// The TLSConfig verifies that the server certifcate is valid and has the correct signature
func DefaultClientTLSConfig(publicKey crypto.PublicKey) (*tls.Config, error) {

	// #nosec G402
	config := &tls.Config{
		MinVersion: tls.VersionTLS13,
		// This is not insecure here. We will verify the cert chain ourselves.
		InsecureSkipVerify: true,
		ClientAuth:         tls.RequireAnyClientCert,
	}

	verifyPeerCertFunc, err := verifyPeerCertificateFunc(publicKey)
	if err != nil {
		return nil, err
	}
	config.VerifyPeerCertificate = verifyPeerCertFunc

	return config, nil
}

func verifyPeerCertificateFunc(expectedPublicKey crypto.PublicKey) (func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error, error) {

	// convert the Flow.crypto key to LibP2P key for easy comparision using LibP2P TLS utils
	expectedLibP2PKey, err := p2p.LibP2PPublicKeyFromFlow(expectedPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate a libp2p key from a Flow key: %w", err)
	}
	remotePeerLibP2PID, err := peer.IDFromPublicKey(expectedLibP2PKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive the libp2p Peer ID from the libp2p public key: %w", err)
	}

	// We're using InsecureSkipVerify, so the verifiedChains parameter will always be empty.
	// We need to parse the certificates ourselves from the raw certs.
	verifyFunc := func(rawCerts [][]byte, _ [][]*x509.Certificate) error {

		chain := make([]*x509.Certificate, len(rawCerts))
		for i := 0; i < len(rawCerts); i++ {
			cert, err := x509.ParseCertificate(rawCerts[i])
			if err != nil {
				return newServerAuthError(err.Error())
			}
			chain[i] = cert
		}

		// libp2ptls.PubKeyFromCertChain verifies the certificate, verifies that the certificate contains the special libp2p
		// extension, extract the remote's public key and finally verifies the signature included in the certificate
		actualLibP2PKey, err := libp2ptls.PubKeyFromCertChain(chain)
		if err != nil {
			return newServerAuthError(err.Error())
		}

		// verify that the public key received is the one that is expected
		if !remotePeerLibP2PID.MatchesPublicKey(actualLibP2PKey) {
			actualKeyHex, err := libP2PKeyToHexString(actualLibP2PKey)
			if err != nil {
				return err
			}
			return newServerAuthError("invalid public key received: expected %s, got %s", expectedPublicKey.String(), actualKeyHex)
		}
		return nil
	}

	return verifyFunc, nil
}

func libP2PKeyToHexString(key lcrypto.PubKey) (string, *ServerAuthError) {
	keyRaw, err := key.Raw()
	if err != nil {
		return "", newServerAuthError(err.Error())
	}
	return hex.EncodeToString(keyRaw), nil
}
