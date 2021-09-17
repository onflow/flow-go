package admin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/rs/zerolog"
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
	grpcAddress  string
	httpAddress  string

	client pb.AdminClient
	conn   *grpc.ClientConn

	cancel context.CancelFunc
}

func TestCommandRunner(t *testing.T) {
	suite.Run(t, new(CommandRunnerSuite))
}

func (suite *CommandRunnerSuite) SetupTest() {
	suite.grpcAddress = fmt.Sprintf("localhost:%s", testingdock.RandomPort(suite.T()))
	suite.httpAddress = fmt.Sprintf("localhost:%s", "49626" /*testingdock.RandomPort(suite.T())*/)
	suite.bootstrapper = NewCommandRunnerBootstrapper()
}

func (suite *CommandRunnerSuite) TearDownTest() {
	err := suite.conn.Close()
	suite.Assert().NoError(err)
	suite.cancel()
	<-suite.runner.Done()
	suite.Assert().Len(suite.runner.commandQ, 0)
	_, ok := <-suite.runner.commandQ
	suite.Assert().False(ok)
}

func (suite *CommandRunnerSuite) SetupCommandRunner() {
	ctx, cancel := context.WithCancel(context.Background())
	suite.cancel = cancel

	logger := zerolog.New(zerolog.NewConsoleWriter())
	suite.runner = suite.bootstrapper.Bootstrap(logger, suite.grpcAddress, WithHTTPServer(suite.httpAddress))
	err := suite.runner.Start(ctx)
	suite.Assert().NoError(err)
	<-suite.runner.Ready()
	go func() {
		select {
		case <-ctx.Done():
			return
		case err := <-suite.runner.Errors():
			suite.Fail("encountered unexpected error", err)
		}
	}()

	conn, err := grpc.Dial(suite.grpcAddress, grpc.WithInsecure())
	suite.Assert().NoError(err)
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

		suite.Assert().EqualValues(data["string"], "foo")
		suite.Assert().EqualValues(data["number"], 123)
		called = true

		return nil
	})

	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["string"] = "foo"
	data["number"] = 123
	val, err := structpb.NewStruct(data)
	suite.Assert().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().NoError(err)
	suite.Assert().True(called)
}

func (suite *CommandRunnerSuite) TestUnimplementedHandler() {
	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["key"] = "value"
	val, err := structpb.NewStruct(data)
	suite.Assert().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().Equal(codes.Unimplemented, status.Code(err))
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
	suite.Assert().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().NoError(err)
	suite.Assert().Equal(calls, 1)

	data["key"] = "blah"
	val, err = structpb.NewStruct(data)
	suite.Assert().NoError(err)
	request.Data = val
	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().Equal(status.Convert(err).Message(), validatorErr.Error())
	suite.Assert().Equal(codes.InvalidArgument, status.Code(err))
	suite.Assert().Equal(calls, 1)
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
	suite.Assert().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().Equal(status.Convert(err).Message(), handlerErr.Error())
	suite.Assert().Equal(codes.Unknown, status.Code(err))
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
	suite.Assert().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().Equal(codes.DeadlineExceeded, status.Code(err))
}

func (suite *CommandRunnerSuite) TestHTTPServer() {
	called := false

	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		suite.Assert().EqualValues(data["key"], "value")
		called = true

		return nil
	})

	suite.SetupCommandRunner()

	url := fmt.Sprintf("http://%s/admin/run_command", suite.httpAddress)
	reqBody := bytes.NewBuffer([]byte(`{"commandName": "foo", "data": {"key": "value"}}`))
	resp, err := http.Post(url, "application/json", reqBody)
	suite.Assert().NoError(err)
	defer resp.Body.Close()

	suite.Assert().True(called)
	suite.Assert().Equal("200 OK", resp.Status)
}

func (suite *CommandRunnerSuite) TestCleanup() {
	suite.bootstrapper.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		<-ctx.Done()
		return ctx.Err()
	})

	suite.SetupCommandRunner()

	data := make(map[string]interface{})
	data["key"] = "value"
	val, err := structpb.NewStruct(data)
	suite.Assert().NoError(err)
	request := &pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}

	var requestsDone sync.WaitGroup
	for i := 0; i < CommandRunnerMaxQueueLength; i++ {
		requestsDone.Add(1)
		go func() {
			defer requestsDone.Done()
			_, err = suite.client.RunCommand(context.Background(), request)
			suite.Assert().Error(err)
		}()
	}

	suite.cancel()

	requestsDone.Wait()
}
