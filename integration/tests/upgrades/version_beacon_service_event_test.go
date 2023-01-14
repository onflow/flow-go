package execution

import (
	"context"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"

	"github.com/stretchr/testify/suite"
)

type TestServiceEventVersionControl struct {
	Suite
}

func (s *TestServiceEventVersionControl) TestEmittingVersionBeaconServiceEvent() {

	// At height 21, run version 0.3.7
	height := uint64(21)
	major := uint8(0)
	minor := uint8(3)
	patch := uint8(7)

	serviceAddress := s.net.Root().Header.ChainID.Chain().ServiceAddress()

	ctx := context.Background()

	env := templates.Environment{
		NodeVersionBeaconAddress: serviceAddress.String(),
	}
	versionBufferScript := templates.GenerateGetVersionUpdateBufferScript(env)

	//Contract should be deployed at bootstrap, so we expect this script to succeed, but ignore the return value
	_, err := s.AccessClient().ExecuteScriptBytes(ctx, versionBufferScript, nil)
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

	err = tx.AddArgument(cadence.NewUInt64(height))
	s.Require().NoError(err)

	err = tx.AddArgument(cadence.NewUInt8(major))
	s.Require().NoError(err)

	err = tx.AddArgument(cadence.NewUInt8(minor))
	s.Require().NoError(err)

	err = tx.AddArgument(cadence.NewUInt8(patch))
	s.Require().NoError(err)

	err = s.AccessClient().SignAndSendTransaction(ctx, tx)
	s.Require().NoError(err)

	txResult, err := s.AccessClient().WaitForSealed(ctx, tx.ID())
	s.Require().NoError(err)

	sealed := s.ReceiptState.WaitForReceiptFromAny(s.T(), flow.Identifier(txResult.BlockID))

	s.Require().Len(sealed.ExecutionResult.ServiceEvents, 1)
	s.Require().IsType(&flow.VersionBeacon{}, sealed.ExecutionResult.ServiceEvents[0].Event)

	versionTable := sealed.ExecutionResult.ServiceEvents[0].Event.(*flow.VersionBeacon)
	s.Require().Equal(uint64(5), versionTable.Sequence)
	s.Require().Len(versionTable.RequiredVersions, 1)
	s.Require().Equal(height, versionTable.RequiredVersions[0].Height)

	semver, err := semver.NewVersion(versionTable.RequiredVersions[0].Version)
	s.Require().NoError(err)
	s.Require().Equal(major, uint8(semver.Major))
	s.Require().Equal(minor, uint8(semver.Minor))
	s.Require().Equal(patch, uint8(semver.Patch))

}

func TestVersionControlServiceEvent(t *testing.T) {
	suite.Run(t, new(TestServiceEventVersionControl))
}
