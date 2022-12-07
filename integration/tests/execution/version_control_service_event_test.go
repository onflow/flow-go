package execution

import (
	"context"
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/suite"
)

type TestServiceEventVersionControl struct {
	Suite
}

func (s *TestServiceEventVersionControl) TestEmittingVersionBeaconServiceEvent() {

	serviceAddress := s.net.Root().Header.ChainID.Chain().ServiceAddress()

	ctx := context.Background()

	env := templates.Environment{
		NodeVersionBeaconAddress: serviceAddress.String(),
	}

	versionBufferScript := templates.GenerateGetVersionUpdateBufferScript(env)

	//height := s.BlockState.HighestFinalizedHeight()

	//Contract should be deployed at bootstrap, so we expect this script to succeed, but ignore the return value
	_, err := s.AccessClient().ExecuteScriptBytes(context.Background(), versionBufferScript, nil)
	s.Require().NoError(err)

	versionTableChangeScript := templates.GenerateChangeVersionTableScript(env)

	latestBlockId, err := s.AccessClient().GetLatestBlockID(ctx)
	s.Require().NoError(err)

	seq := s.AccessClient().GetSeqNumber()

	tx := sdk.NewTransaction().
		SetScript(versionTableChangeScript).
		SetReferenceBlockID(sdk.Identifier(latestBlockId)).
		SetProposalKey(sdk.Address(serviceAddress), 0, seq).
		SetPayer(sdk.Address(serviceAddress)).
		AddAuthorizer(sdk.Address(serviceAddress))

	//args
	//  height: UInt64,
	//  newMajor: UInt8,
	//  newMinor: UInt8,
	//  newPatch: UInt8,
	//err = tx.AddArgument(cadence.NewUInt64(uint64(21)))
	//s.Require().NoError(err)
	//
	//err = tx.AddArgument(cadence.NewUInt8(uint8(0)))
	//s.Require().NoError(err)
	//
	//err = tx.AddArgument(cadence.NewUInt8(uint8(3)))
	//s.Require().NoError(err)
	//
	//err = tx.AddArgument(cadence.NewUInt8(uint8(7)))
	//s.Require().NoError(err)

	//err = s.AccessClient().SignAndSendTransaction(ctx, tx)
	//s.Require().NoError(err)

	fmt.Println("WAITING NOW!!! txSigned " + tx.ID().String())

	//result, err := s.AccessClient().WaitForSealed(ctx, tx.ID())

	reasult := s.BlockState.WaitForSealed(s.T(), 200)
	spew.Dump(reasult)

	//

	//
	//sealed := s.BlockState.WaitForSealed(s.T(), results.BlockHeight)
	//
	//spew.Dump(sealed)

	//receipt := s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(result.BlockID))

	//spew.Dump(receipt)

	//sealed, err := s.AccessClient().WaitForSealed(ctx, tx.ID())
	//s.Require().NoError(err)
	//
	//spew.Dump(sealed)

	//
	//s.AccessClient().SendTransaction()
	//
	//enContainer := s.net.ContainerByID(s.exe1ID)
	//
	//// make sure stop at height admin command is available
	//commandsList := AdminCommandListCommands{}
	//err := s.SendExecutionAdminCommand(context.Background(), "list-commands", struct{}{}, &commandsList)
	//require.NoError(s.T(), err)
	//
	//require.Contains(s.T(), commandsList, "stop-at-height")
	//
	//// wait for some blocks being finalized
	//s.BlockState.WaitForHighestFinalizedProgress(s.T(), 2)
	//
	//currentFinalized := s.BlockState.HighestFinalizedHeight()
	//
	//// stop in 5 blocks
	//stopHeight := currentFinalized + 5
	//
	//stopAtHeightRequest := StopAtHeightRequest{
	//	Height: stopHeight,
	//	Crash:  true,
	//}
	//
	//var commandResponse string
	//err = s.SendExecutionAdminCommand(context.Background(), "stop-at-height", stopAtHeightRequest, &commandResponse)
	//require.NoError(s.T(), err)
	//
	//require.Equal(s.T(), "ok", commandResponse)
	//
	//shouldExecute := s.BlockState.WaitForBlocksByHeight(s.T(), stopHeight-1)
	//shouldNotExecute := s.BlockState.WaitForBlocksByHeight(s.T(), stopHeight)
	//
	//s.ReceiptState.WaitForReceiptFrom(s.T(), shouldExecute[0].Header.ID(), s.exe1ID)
	//s.ReceiptState.WaitForNoReceiptFrom(s.T(), 5*time.Second, shouldNotExecute[0].Header.ID(), s.exe1ID)
	//
	//err = enContainer.WaitForContainerStopped(10 * time.Second)

	//s.BlockState.WaitForBlocksByHeight(s.T(), height+10)

}

func TestVersionControlServiceEvent(t *testing.T) {
	suite.Run(t, new(TestServiceEventVersionControl))
}
