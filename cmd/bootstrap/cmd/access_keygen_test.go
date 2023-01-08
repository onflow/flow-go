package cmd

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	golog "log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestAccessKeyFileCreated(t *testing.T) {
	unittest.RunWithTempDir(t, func(bootDir string) {
		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// generate test node keys
		flagRole = "access"
		flagAddress = "localhost:1234"
		flagOutdir = bootDir

		keyCmdRun(nil, nil)

		// find the node-info.priv.json file
		// the path includes a random hex string, so we need to find it
		err := filepath.Walk(bootDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.Name() == "node-info.priv.json" {
				flagNodeInfoFile = path
			}
			return nil
		})
		require.NoError(t, err)

		flagCommonName = "unittest.onflow.org"
		flagOutputKeyFile = filepath.Join(bootDir, "test-access-key.key")
		flagOutputCertFile = filepath.Join(bootDir, "test-access-key.cert")

		golog.Println(bootDir)

		// run command with flags
		accessKeyCmdRun(nil, nil)

		// make sure key/cert files exists (regex checks this too)
		require.FileExists(t, flagOutputKeyFile)
		require.FileExists(t, flagOutputCertFile)

		// decode key and cert and make sure they match
		keyData, err := io.ReadFile(flagOutputKeyFile)
		require.NoError(t, err)

		certData, err := io.ReadFile(flagOutputCertFile)
		require.NoError(t, err)

		privKey, cert := decodeKeys(t, keyData, certData)

		// check that the public key from the cert matches the private key
		ecdsaPubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
		require.True(t, ok)
		require.Equal(t, privKey.PublicKey, *ecdsaPubKey)

		// check that the common name is correct
		assert.Equal(t, flagCommonName, cert.Subject.CommonName, "expected %s, got %s", flagCommonName, cert.Subject.CommonName)
		assert.Equal(t, flagCommonName, cert.Issuer.CommonName, "expected %s, got %s", flagCommonName, cert.Issuer.CommonName)
	})
}

func decodeKeys(t *testing.T, pemEncoded []byte, pemEncodedPub []byte) (*ecdsa.PrivateKey, *x509.Certificate) {
	block, _ := pem.Decode(pemEncoded)
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	require.NoError(t, err)

	blockPub, _ := pem.Decode(pemEncodedPub)
	cert, err := x509.ParseCertificate(blockPub.Bytes)
	require.NoError(t, err)

	return privateKey, cert
}
