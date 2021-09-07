package admin

import (
	"context"
	"fmt"
	"testing"

	"github.com/dapperlabs/testingdock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/onflow/flow-go/admin/admin"
)

// test register / unregister handler
// test register / unregister validator
// test handler error
// test validation error
// test non-existent command
// test shutdown cleanup

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

	conn, err := grpc.Dial(suite.address)
	suite.Assert().NoError(err)
	suite.client = pb.NewAdminClient(conn)
}

func (suite *CommandRunnerSuite) TearDownTest() {
	err := suite.conn.Close()
	suite.Assert().NoError(err)
	suite.cancel()
	<-suite.runner.Done()
}

func (suite *CommandRunnerSuite) TestRegisterHandler() {
	suite.runner.RegisterHandler("foo", func(ctx context.Context, data map[string]interface{}) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// TODO
	})

	data := make(map[string]interface{})
	data["string"] = "foo"
	data["number"] = 123
	val, err := structpb.NewStruct(data)
	suite.Assert().NoError(err)

	request := pb.RunCommandRequest{
		CommandName: "foo",
		Data:        val,
	}
	suite.client.RunCommand(ctx, request)

}
