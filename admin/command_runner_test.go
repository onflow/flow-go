package admin

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/onflow/flow-go/admin/admin"
)

type CommandRunnerSuite struct {
	suite.Suite

	runner       *CommandRunner
	bootstrapper *CommandRunnerBootstrapper
	httpAddress  string

	client pb.AdminClient
	conn   *grpc.ClientConn

	cancel context.CancelFunc
}

func TestCommandRunner(t *testing.T) {
	suite.Run(t, new(CommandRunnerSuite))
}

func (suite *CommandRunnerSuite) SetupTest() {
	suite.httpAddress = fmt.Sprintf("localhost:%s", testingdock.RandomPort(suite.T()))
	suite.bootstrapper = NewCommandRunnerBootstrapper()
}

func (suite *CommandRunnerSuite) TearDownTest() {
	err := suite.conn.Close()
	suite.NoError(err)
	suite.cancel()
	<-suite.runner.Done()
}

func (suite *CommandRunnerSuite) SetupCommandRunner(opts ...CommandRunnerOption) {
	ctx, cancel := context.WithCancel(context.Background())
	suite.cancel = cancel

	logger := zerolog.New(zerolog.NewConsoleWriter())
	suite.runner = suite.bootstrapper.Bootstrap(logger, suite.httpAddress, opts...)
	err := suite.runner.Start(ctx)
	suite.NoError(err)
	<-suite.runner.Ready()
	go func() {
		select {
		case <-ctx.Done():
			return
		case err := <-suite.runner.Errors():
			suite.Fail("encountered unexpected error", err)
		}
	}()

	conn, err := grpc.Dial("unix:///"+suite.runner.grpcAddress, grpc.WithInsecure())
	suite.NoError(err)
	suite.conn = conn
	suite.client = pb.NewAdminClient(conn)
}

func (suite *CommandRunnerSuite) TestHandler() {
	called := false

	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		suite.EqualValues(data["string"], "foo")
		suite.EqualValues(data["number"], 123)
		called = true

		return nil
	})

	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["string"] = "foo"
	data["number"] = 123
	val, err := structpb.NewStruct(data)
	suite.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.NoError(err)
	suite.True(called)
}

func (suite *CommandRunnerSuite) TestUnimplementedHandler() {
	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["key"] = "value"
	val, err := structpb.NewStruct(data)
	suite.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Equal(codes.Unimplemented, status.Code(err))
}

func (suite *CommandRunnerSuite) TestValidator() {
	calls := 0

	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		calls += 1

		return nil
	})

	validatorErr := errors.New("unexpected value")
	suite.bootstrapper.RegisterValidator("foo", func(data map[string]interface{}) error {
		if data["key"] != "value" {
			return validatorErr
		}
		return nil
	})

	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["key"] = "value"
	val, err := structpb.NewStruct(data)
	suite.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.NoError(err)
	suite.Equal(calls, 1)

	data["key"] = "blah"
	val, err = structpb.NewStruct(data)
	suite.NoError(err)
	request.Data = val
	_, err = suite.client.RunCommand(ctx, request)
	suite.Equal(status.Convert(err).Message(), validatorErr.Error())
	suite.Equal(codes.InvalidArgument, status.Code(err))
	suite.Equal(calls, 1)
}

func (suite *CommandRunnerSuite) TestHandlerError() {
	handlerErr := errors.New("handler error")
	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		return handlerErr
	})

	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["key"] = "value"
	val, err := structpb.NewStruct(data)
	suite.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Equal(status.Convert(err).Message(), handlerErr.Error())
	suite.Equal(codes.Unknown, status.Code(err))
}

func (suite *CommandRunnerSuite) TestTimeout() {
	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		<-ctx.Done()
		return ctx.Err()
	})

	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["key"] = "value"
	val, err := structpb.NewStruct(data)
	suite.NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Equal(codes.DeadlineExceeded, status.Code(err))
}

func (suite *CommandRunnerSuite) TestHTTPServer() {
	called := false

	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		suite.EqualValues(data["key"], "value")
		called = true

		return nil
	})

	suite.SetupCommandRunner()

	url := fmt.Sprintf("http://%s/admin/run_command", suite.httpAddress)
	reqBody := bytes.NewBuffer([]byte(`{"commandName": "foo", "data": {"key": "value"}}`))
	resp, err := http.Post(url, "application/json", reqBody)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.True(called)
	suite.Equal("200 OK", resp.Status)
}

func generateCerts(t *testing.T) (tls.Certificate, *x509.CertPool, tls.Certificate, *x509.CertPool) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Dapper Labs, Inc."},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 180),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caPrivKeyBytes, err := x509.MarshalECPrivateKey(caPrivKey)
	require.NoError(t, err)
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)
	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	require.NoError(t, err)
	caPrivKeyPem := new(bytes.Buffer)
	err = pem.Encode(caPrivKeyPem, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: caPrivKeyBytes,
	})
	require.NoError(t, err)

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Dapper Labs, Inc."},
		},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour * 24 * 180),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serverPrivKeyBytes, err := x509.MarshalECPrivateKey(serverPrivKey)
	require.NoError(t, err)
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, ca, &serverPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)
	serverCertPEM := new(bytes.Buffer)
	err = pem.Encode(serverCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertBytes,
	})
	require.NoError(t, err)
	serverPrivKeyPem := new(bytes.Buffer)
	err = pem.Encode(serverPrivKeyPem, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: serverPrivKeyBytes,
	})
	require.NoError(t, err)
	serverCert, err := tls.X509KeyPair(serverCertPEM.Bytes(), serverPrivKeyPem.Bytes())
	require.NoError(t, err)
	serverCert.Leaf, err = x509.ParseCertificate(serverCert.Certificate[0])
	require.NoError(t, err)
	serverCertPool := x509.NewCertPool()
	serverCertPool.AddCert(serverCert.Leaf)

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"Dapper Labs, Inc."},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour * 24 * 180),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientPrivKeyBytes, err := x509.MarshalECPrivateKey(clientPrivKey)
	require.NoError(t, err)
	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientTemplate, ca, &clientPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)
	clientCertPEM := new(bytes.Buffer)
	err = pem.Encode(clientCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertBytes,
	})
	require.NoError(t, err)
	clientPrivKeyPem := new(bytes.Buffer)
	err = pem.Encode(clientPrivKeyPem, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: clientPrivKeyBytes,
	})
	require.NoError(t, err)
	clientCert, err := tls.X509KeyPair(clientCertPEM.Bytes(), clientPrivKeyPem.Bytes())
	require.NoError(t, err)
	clientCert.Leaf, err = x509.ParseCertificate(clientCert.Certificate[0])
	require.NoError(t, err)
	clientCertPool := x509.NewCertPool()
	clientCertPool.AddCert(clientCert.Leaf)

	return serverCert, serverCertPool, clientCert, clientCertPool
}

func (suite *CommandRunnerSuite) TestTLS() {
	called := false

	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		suite.EqualValues(data["key"], "value")
		called = true

		return nil
	})

	serverCert, serverCertPool, clientCert, clientCertPool := generateCerts(suite.T())
	serverConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCertPool,
	}
	clientConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      serverCertPool,
	}

	suite.SetupCommandRunner(WithTLS(serverConfig))

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientConfig,
		},
	}
	url := fmt.Sprintf("https://%s/admin/run_command", suite.httpAddress)
	reqBody := bytes.NewBuffer([]byte(`{"commandName": "foo", "data": {"key": "value"}}`))
	resp, err := client.Post(url, "application/json", reqBody)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.True(called)
	suite.Equal("200 OK", resp.Status)
}
