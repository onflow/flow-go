package grpcutils

import (
	"crypto/x509"
	"testing"
	"time"

	libp2ptls "github.com/libp2p/go-libp2p-tls"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/utils/unittest"
)

const year = 365 * 24 * time.Hour

// TestCertificateGeneration tests the X509Certificate certificate generation
func TestCertificateGeneration(t *testing.T) {
	// test key
	key := unittest.NetworkingPrivKeyFixture()

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
	libp2pKey, err := keyutils.LibP2PPrivKeyFromFlow(key)
	expectedKey := libp2pKey.GetPublic()
	require.NoError(t, err)

	// assert that the public key in the cert matches the test public key
	require.True(t, expectedKey.Equals(pubKey))

	// assert that the the cert is valid for at least an year starting from now
	now := time.Now()
	require.True(t, now.After(cert.NotBefore))
	require.True(t, cert.NotAfter.After(now.Add(year)))
}

// TestPeerCertificateVerification tests that the verifyPeerCertificate function correctly verifies a server cert
func TestPeerCertificateVerification(t *testing.T) {
	// test key
	key := unittest.NetworkingPrivKeyFixture()

	// generate the certificate from the key
	certs, err := X509Certificate(key)
	require.NoError(t, err)

	// derive the verification function
	verifyFunc, err := verifyPeerCertificateFunc(key.PublicKey())
	require.NoError(t, err)

	t.Run("happy path - certificate validation passes", func(t *testing.T) {
		// call the verify function and assert that the certificate is validated
		err = verifyFunc(certs.Certificate, nil)
		require.NoError(t, err)
	})

	t.Run("certificate validation fails for a different public key", func(t *testing.T) {
		// generate another key and certificate
		key2 := unittest.NetworkingPrivKeyFixture()
		certs2, err := X509Certificate(key2)
		require.NoError(t, err)

		// call the verify function again and assert that the certificate with a different public key is not validated
		// and a ServerAuthError is thrown
		err = verifyFunc(certs2.Certificate, nil)
		require.True(t, IsServerAuthError(err))
	})
}
