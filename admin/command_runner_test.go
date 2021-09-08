package admin

import (
	"context"
	"errors"
	"fmt"
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

	runner  *CommandRunner
	address string

	client pb.AdminClient
	conn   *grpc.ClientConn

	cancel context.CancelFunc
}

func TestClusterCommittee(t *testing.T) {
	suite.Run(t, new(CommandRunnerSuite))
}

func (suite *CommandRunnerSuite) SetupTest() {
	ctx, cancel := context.WithCancel(context.Background())
	suite.cancel = cancel

	logger := zerolog.New(zerolog.NewConsoleWriter())
	suite.address = fmt.Sprintf("localhost:%s", testingdock.RandomPort(suite.T()))
	suite.runner = NewCommandRunner(logger, suite.address)
	err := suite.runner.Start(ctx)
	suite.Assert().NoError(err)
	<-suite.runner.Ready()

	conn, err := grpc.Dial(suite.address, grpc.WithInsecure())
	suite.Assert().NoError(err)
	suite.conn = conn
	suite.client = pb.NewAdminClient(conn)
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

func (suite *CommandRunnerSuite) TestHandler() {
	called := false

	suite.runner.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
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

	suite.runner.UnregisterHandler("foo")
	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().Equal(codes.Unimplemented, status.Code(err))
}

func (suite *CommandRunnerSuite) TestValidator() {
	calls := 0

	suite.runner.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		calls += 1

		return nil
	})

	validatorErr := errors.New("unexpected value")
	suite.runner.RegisterValidator("foo", func(data map[string]interface{}) error {
		if data["key"] != "value" {
			return validatorErr
		}
		return nil
	})

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

	suite.runner.UnregisterValidator("foo")

	val, err = structpb.NewStruct(data)
	suite.Assert().NoError(err)
	request.Data = val
	_, err = suite.client.RunCommand(ctx, request)
	suite.Assert().NoError(err)
	suite.Assert().Equal(calls, 2)
}

func (suite *CommandRunnerSuite) TestHandlerError() {
	handlerErr := errors.New("handler error")
	suite.runner.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		return handlerErr
	})

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
	suite.runner.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		<-ctx.Done()
		return ctx.Err()
	})

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

func (suite *CommandRunnerSuite) TestCleanup() {
	suite.runner.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		<-ctx.Done()
		return ctx.Err()
	})

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
