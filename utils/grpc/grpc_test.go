package grpcutils

import (
	"crypto/x509"
	"testing"
	"time"

	libp2ptls "github.com/libp2p/go-libp2p-tls"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

const year = 365 * 24 * time.Hour

// TestCertificateGeneration tests the X509Certificate certificate generation
func TestCertificateGeneration(t *testing.T) {
	// test key
	key, err := unittest.NetworkingKey()
	require.NoError(t, err)

	// generate the certificate from the key
	certs, err := X509Certificate(key)
	require.NoError(t, err)

	// assert that only one certificate is generated
	require.Len(t, certs.Certificate, 1)

	// parse the cert
	cert, err := x509.ParseCertificate(certs.Certificate[0])
	require.NoError(t, err)

	// extract the public key from the cert
	pubKey, err := libp2ptls.PubKeyFromCertChain([]*x509.Certificate{cert})
	require.NoError(t, err)

	// convert the test key to a libp2p key for easy comparision
	libp2pKey, err := p2p.PrivKey(key)
	expectedKey := libp2pKey.GetPublic()
	require.NoError(t, err)

	// assert that the public key in the cert matches the test public key
	require.True(t, expectedKey.Equals(pubKey))

	// assert that the the cert is valid for at least an year starting from now
	now := time.Now()
	require.True(t, now.After(cert.NotBefore))
	require.True(t, cert.NotAfter.After(now.Add(year)))
}
