package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type TestServiceEventVersionControl struct {
	Suite
}

func (s *TestServiceEventVersionControl) TestEmittingVersionBeaconServiceEvent() {

	//serviceAddress := s.net.Root().Header.ChainID.Chain().ServiceAddress()

	//ctx := context.Background()

	//env := templates.Environment{
	//	NodeVersionBeaconAddress: serviceAddress.String(),
	//}

	//versionBufferScript := templates.GenerateGetVersionUpdateBufferScript(env)

	for i := 0; i < 120; i++ {
		header, err := s.AccessClient().GetLatestSealedBlockHeader(context.Background())
		s.Require().NoError(err)

		fmt.Printf("Sealed height is %d\n", header.Height)

		time.Sleep(1 * time.Second)
	}

	//height := s.BlockState.HighestFinalizedHeight()

	// Contract should be deployed at bootstrap, so we expect this script to succeed, but ignore the return value
	//_, err := s.AccessClient().ExecuteScriptBytes(context.Background(), versionBufferScript, nil)
	//
	//s.Require().NoError(err)

	//latestBlockId, err := s.AccessClient().GetLatestBlockID(ctx)
	//s.Require().NoError(err)

	//seq := s.AccessClient().GetSeqNumber()
	//
	//tx := sdk.NewTransaction().
	//	SetScript(versionBufferScript).
	//	SetReferenceBlockID(sdk.Identifier(latestBlockId)).
	//	SetProposalKey(sdk.Address(serviceAddress), 0, seq).
	//	SetPayer(sdk.Address(serviceAddress)).
	//	AddAuthorizer(sdk.Address(serviceAddress))
	//
	//fmt.Printf("TX created %s\n", tx.ID())
	//
	//err = s.AccessClient().SignAndSendTransaction(ctx, tx)
	//s.Require().NoError(err)

	//results, err := s.AccessClient().WaitForAtLeastStatus(ctx, tx.ID(), sdk.TransactionStatusExecuted)
	//
	//spew.Dump(results)
	//
	//fmt.Println("WAITING NOW!!! txSigned " + tx.ID().String())
	//
	//sealed := s.BlockState.WaitForSealed(s.T(), results.BlockHeight)
	//
	//spew.Dump(sealed)

	s.BlockState.WaitForSealed(s.T(), 15)

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
