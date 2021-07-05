package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p-core/peer"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/network/p2p"
)

func main() {

	accessNodeAddress := "localhost:9000"

	bytes, err := hex.DecodeString("691ef1777846fb6e0ad15d5dce77c10617acafab16cf6635df4a50dd29e6e9d448e22a0eee587904165153a8e5a44df92fec898a2134b2efe663a691b4bbcbe8")
	if err != nil {
		log.Fatalf("failed to decode public key: %v", err)
	}
	publicFlowNetworkingKey, err := crypto.DecodePublicKey(crypto.ECDSAP256, bytes)
	if err != nil {
		log.Fatalf("failed to create the Flow networking public key: %v", err)
	}
	fmt.Printf(" using public key %s for the remote node\n", publicFlowNetworkingKey.String())

	publicLibp2pNetworkingKey, err := p2p.PublicKey(publicFlowNetworkingKey)
	if err != nil {
		log.Fatalf("failed to generate a libp2p key from a Flow key: %v", err)
	}
	remotePeerLibP2PID, err := peer.IDFromPublicKey(publicLibp2pNetworkingKey)
	if err != nil {
		log.Fatalf("failed to derive the libp2p Peer ID from the libp2p public key: %v", err)
	}

	tlsConfig, err := ConfigForPeer(remotePeerLibP2PID)
	if err != nil {
		log.Fatalf("failed to derive the TLS Config for the remote peer: %v", err)
	}

	conn, err := grpc.Dial(accessNodeAddress, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	// create an AccessAPIClient
	grpcClient := access.NewAccessAPIClient(conn)

	ctx := context.Background()

	request := access.GetLatestBlockRequest{
	}

	response, err := grpcClient.GetLatestBlock(ctx, &request)
	if err != nil {
		panic(err)
	}

	fmt.Println("latest block height: ")
	fmt.Println(response.GetBlock().String())
}

func ConfigForPeer(remote peer.ID) (*tls.Config, error) {

	config := &tls.Config{
		MinVersion:               tls.VersionTLS13,
		InsecureSkipVerify:       true, // This is not insecure here. We will verify the cert chain ourselves.
		ClientAuth:               tls.RequireAnyClientCert,
	}
	// We're using InsecureSkipVerify, so the verifiedChains parameter will always be empty.
	// We need to parse the certificates ourselves from the raw certs.
	config.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {

		chain := make([]*x509.Certificate, len(rawCerts))
		for i := 0; i < len(rawCerts); i++ {
			cert, err := x509.ParseCertificate(rawCerts[i])
			if err != nil {
				return err
			}
			chain[i] = cert
		}

		pubKey, err := libp2ptls.PubKeyFromCertChain(chain)
		if err != nil {
			return err
		}
		if remote != "" && !remote.MatchesPublicKey(pubKey) {
			peerID, err := peer.IDFromPublicKey(pubKey)
			if err != nil {
				peerID = peer.ID(fmt.Sprintf("(not determined: %s)", err.Error()))
			}
			return fmt.Errorf("peer IDs don't match: expected %s, got %s", remote, peerID)
		}
		return nil
	}
	return config, nil
}
