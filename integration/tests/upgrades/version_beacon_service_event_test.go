package upgrades

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
	// version 0.3.7
	major := uint8(0)
	minor := uint8(3)
	patch := uint8(7)
	preRelease := ""

	serviceAddress := s.net.Root().Header.ChainID.Chain().ServiceAddress()

	ctx := context.Background()

	env := templates.Environment{
		NodeVersionBeaconAddress: serviceAddress.String(),
	}
	freezePeriodScript := templates.GenerateGetVersionBoundaryFreezePeriodScript(env)

	// Contract should be deployed at bootstrap,
	// so we expect this script to succeed, but ignore the return value
	freezePeriodRaw, err := s.AccessClient().
		ExecuteScriptBytes(ctx, freezePeriodScript, nil)
	s.Require().NoError(err)

	freezePeriod := uint64(0)

	if cadenceBuffer, is := freezePeriodRaw.(cadence.UInt64); is {
		freezePeriod = cadenceBuffer.ToGoValue().(uint64)
	} else {
		s.Require().Failf(
			"version freezePeriod script returned unknown type",
			"%t",
			freezePeriodRaw,
		)
	}

	s.Run("should fail adding version boundary inside the freeze period", func() {

		height := freezePeriod / 2

		txResult := s.sendSetVersionBoundaryTransaction(
			ctx,
			env,
			versionBoundary{
				Major:       major,
				Minor:       minor,
				Patch:       patch,
				PreRelease:  preRelease,
				BlockHeight: height,
			})
		s.Require().Error(txResult.Error)

		sealed := s.ReceiptState.WaitForReceiptFromAny(
			s.T(),
			flow.Identifier(txResult.BlockID))
		s.Require().Len(sealed.ExecutionResult.ServiceEvents, 0)
	})

	s.Run("should add version boundary after the freeze period", func() {

		// make sure target height is correct
		// the height at which the version change will take effect should be after
		// the current height + the freeze period
		height := freezePeriod + 200

		txResult := s.sendSetVersionBoundaryTransaction(
			ctx,
			env,
			versionBoundary{
				Major:       major,
				Minor:       minor,
				Patch:       patch,
				PreRelease:  preRelease,
				BlockHeight: height,
			})
		s.Require().NoError(txResult.Error)

		sealed := s.ReceiptState.WaitForReceiptFromAny(
			s.T(),
			flow.Identifier(txResult.BlockID))

		s.Require().Len(sealed.ExecutionResult.ServiceEvents, 1)
		s.Require().IsType(
			&flow.VersionBeacon{},
			sealed.ExecutionResult.ServiceEvents[0].Event)

		versionTable := sealed.ExecutionResult.ServiceEvents[0].Event.(*flow.VersionBeacon)
		// this should be the second ever emitted
		// the first was emitted at bootstrap
		s.Require().Equal(uint64(1), versionTable.Sequence)
		s.Require().Len(versionTable.VersionBoundaries, 2)

		// zeroth boundary should be present, as it is the one we should be on
		s.Require().Equal(uint64(0), versionTable.VersionBoundaries[0].BlockHeight)

		version, err := semver.NewVersion(versionTable.VersionBoundaries[0].Version)
		s.Require().NoError(err)
		s.Require().Equal(uint8(0), uint8(version.Major))
		s.Require().Equal(uint8(0), uint8(version.Minor))
		s.Require().Equal(uint8(0), uint8(version.Patch))

		s.Require().Equal(height, versionTable.VersionBoundaries[1].BlockHeight)

		version, err = semver.NewVersion(versionTable.VersionBoundaries[1].Version)
		s.Require().NoError(err)
		s.Require().Equal(major, uint8(version.Major))
		s.Require().Equal(minor, uint8(version.Minor))
		s.Require().Equal(patch, uint8(version.Patch))
	})

}

type versionBoundary struct {
	BlockHeight uint64
	Major       uint8
	Minor       uint8
	Patch       uint8
	PreRelease  string
}

func (s *TestServiceEventVersionControl) sendSetVersionBoundaryTransaction(
	ctx context.Context,
	env templates.Environment,
	boundary versionBoundary,
) *sdk.TransactionResult {
	serviceAddress := s.net.Root().Header.ChainID.Chain().ServiceAddress()

	versionTableChangeScript := templates.GenerateSetVersionBoundaryScript(env)

	latestBlockId, err := s.AccessClient().GetLatestBlockID(ctx)
	s.Require().NoError(err)
	seq := s.AccessClient().GetSeqNumber()

	tx := sdk.NewTransaction().
		SetScript(versionTableChangeScript).
		SetReferenceBlockID(sdk.Identifier(latestBlockId)).
		SetProposalKey(sdk.Address(serviceAddress), 0, seq).
		SetPayer(sdk.Address(serviceAddress)).
		AddAuthorizer(sdk.Address(serviceAddress))

	// args
	// newMajor: UInt8,
	// newMinor: UInt8,
	// newPatch: UInt8,
	// newPreRelease: String?,
	// targetBlockHeight: UInt64

	err = tx.AddArgument(cadence.NewUInt8(boundary.Major))
	s.Require().NoError(err)

	err = tx.AddArgument(cadence.NewUInt8(boundary.Minor))
	s.Require().NoError(err)

	err = tx.AddArgument(cadence.NewUInt8(boundary.Patch))
	s.Require().NoError(err)

	preReleaseCadenceString, err := cadence.NewString(boundary.PreRelease)
	s.Require().NoError(err)
	err = tx.AddArgument(preReleaseCadenceString)
	s.Require().NoError(err)

	err = tx.AddArgument(cadence.NewUInt64(boundary.BlockHeight))
	s.Require().NoError(err)

	err = s.AccessClient().SignAndSendTransaction(ctx, tx)
	s.Require().NoError(err)

	txResult, err := s.AccessClient().WaitForSealed(ctx, tx.ID())
	s.Require().NoError(err)
	return txResult
}

func TestVersionControlServiceEvent(t *testing.T) {
	suite.Run(t, new(TestServiceEventVersionControl))
}
