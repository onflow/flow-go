package cmd

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/utils/grpcutils"
)

const certValidityPeriod = 100 * 365 * 24 * time.Hour // ~100 years

var (
	flagSANs           string
	flagCommonName     string
	flagNodeInfoFile   string
	flagOutputKeyFile  string
	flagOutputCertFile string
)

var accessKeyCmd = &cobra.Command{
	Use:   "access-keygen",
	Short: "Generate access node grpc TLS key and certificate",
	Run:   accessKeyCmdRun,
}

func init() {
	rootCmd.AddCommand(accessKeyCmd)

	accessKeyCmd.Flags().StringVar(&flagNodeInfoFile, "node-info", "", "path to node's node-info.priv.json file")
	_ = accessKeyCmd.MarkFlagRequired("node-info")

	accessKeyCmd.Flags().StringVar(&flagOutputKeyFile, "key", "./access-tls.key", "path to output private key file")
	accessKeyCmd.Flags().StringVar(&flagOutputCertFile, "cert", "./access-tls.crt", "path to output certificate file")
	accessKeyCmd.Flags().StringVar(&flagCommonName, "cn", "", "common name to include in the certificate")
	accessKeyCmd.Flags().StringVar(&flagSANs, "sans", "", "subject alternative names to include in the certificate, comma separated")
}

// accessKeyCmdRun generate an Access node TLS key and certificate
func accessKeyCmdRun(_ *cobra.Command, _ []string) {
	networkKey, err := loadNetworkKey(flagNodeInfoFile)
	if err != nil {
		log.Fatal().Msgf("could not load node-info file: %v", err)
	}

	certTmpl, err := defaultCertTemplate()
	if err != nil {
		log.Fatal().Msgf("could not create certificate template: %v", err)
	}

	if flagCommonName != "" {
		log.Info().Msgf("using cn: %s", flagCommonName)
		certTmpl.Subject.CommonName = flagCommonName
	}

	if flagSANs != "" {
		log.Info().Msgf("using SANs: %s", flagSANs)
		certTmpl.DNSNames = strings.Split(flagSANs, ",")
	}

	cert, err := grpcutils.X509Certificate(networkKey, grpcutils.WithCertTemplate(certTmpl))
	if err != nil {
		log.Fatal().Msgf("could not generate key pair: %v", err)
	}

	// write cert and private key to disk
	keyBytes, err := x509.MarshalECPrivateKey(cert.PrivateKey.(*ecdsa.PrivateKey))
	if err != nil {
		log.Fatal().Msgf("could not encode private key: %v", err)
	}

	err = os.WriteFile(flagOutputKeyFile, pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}), 0600)
	if err != nil {
		log.Fatal().Msgf("could not write private key: %v", err)
	}

	err = os.WriteFile(flagOutputCertFile, pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	}), 0600)
	if err != nil {
		log.Fatal().Msgf("could not write certificate: %v", err)
	}
}

func loadNetworkKey(nodeInfoPath string) (crypto.PrivateKey, error) {
	data, err := os.ReadFile(nodeInfoPath)
	if err != nil {
		return nil, fmt.Errorf("could not read private node info (path=%s): %w", nodeInfoPath, err)
	}

	var info bootstrap.NodeInfoPriv
	err = json.Unmarshal(data, &info)
	if err != nil {
		return nil, fmt.Errorf("could not parse private node info (path=%s): %w", nodeInfoPath, err)
	}

	return info.NetworkPrivKey.PrivateKey, nil
}

func defaultCertTemplate() (*x509.Certificate, error) {
	bigNum := big.NewInt(1 << 62)
	sn, err := rand.Int(rand.Reader, bigNum)
	if err != nil {
		return nil, err
	}

	subjectSN, err := rand.Int(rand.Reader, bigNum)
	if err != nil {
		return nil, err
	}

	return &x509.Certificate{
		SerialNumber: sn,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(certValidityPeriod),
		// According to RFC 3280, the issuer field must be set,
		// see https://datatracker.ietf.org/doc/html/rfc3280#section-4.1.2.4.
		Subject: pkix.Name{SerialNumber: subjectSN.String()},
	}, nil
}
