package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

func dynamicJoinFlagsFixture() (string, string, flow.EpochPhase, uint64) {
	return unittest.NetworkingPrivKeyFixture().PublicKey().String(), "access_1:9001", flow.EpochPhaseSetup, 1
}

func getMockSnapshot(t *testing.T, epochCounter uint64, phase flow.EpochPhase) *protocolmock.Snapshot {
	currentEpoch := new(protocolmock.Epoch)
	currentEpoch.On("Counter").Return(epochCounter, nil)

	epochQuery := mocks.NewEpochQuery(t, epochCounter)
	epochQuery.Add(currentEpoch)

	snapshot := new(protocolmock.Snapshot)
	snapshot.On("Epochs").Return(epochQuery)
	snapshot.On("Phase").Return(phase, nil)

	return snapshot
}

// TestValidateDynamicStartupFlags tests validation of dynamic-startup-* CLI flags
func TestValidateDynamicStartupFlags(t *testing.T) {
	t.Run("should return nil if all flags are valid", func(t *testing.T) {
		pub, address, phase, _ := dynamicJoinFlagsFixture()
		err := ValidateDynamicStartupFlags(pub, address, phase)
		require.NoError(t, err)
	})

	t.Run("should return error if access network key is not valid ECDSA_P256 public key", func(t *testing.T) {
		_, address, phase, _ := dynamicJoinFlagsFixture()
		err := ValidateDynamicStartupFlags("0xKEY", address, phase)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid flag --dynamic-startup-access-publickey")
	})

	t.Run("should return error if access address is empty", func(t *testing.T) {
		pub, _, phase, _ := dynamicJoinFlagsFixture()
		err := ValidateDynamicStartupFlags(pub, "", phase)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid flag --dynamic-startup-access-address")
	})

	t.Run("should return error if startup epoch phase is invalid", func(t *testing.T) {
		pub, address, _, _ := dynamicJoinFlagsFixture()
		err := ValidateDynamicStartupFlags(pub, address, -1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid flag --dynamic-startup-startup-epoch-phase")
	})
}

// TestGetSnapshotAtEpochAndPhase ensures the target start epoch and phase conditions are met before returning a snapshot
// for a node to bootstrap with by asserting the expected number of warn/info log messages are output and the expected
// snapshot is returned
func TestGetSnapshotAtEpochAndPhase(t *testing.T) {
	t.Run("should retry until a snapshot is observed with desired epoch/phase", func(t *testing.T) {
		// the snapshot we will use to force GetSnapshotAtEpochAndPhase to retry
		oldSnapshot := getMockSnapshot(t, 0, flow.EpochPhaseStaking)

		// the snapshot that will return target counter and phase
		expectedSnapshot := getMockSnapshot(t, 1, flow.EpochPhaseSetup)

		// setup mock get snapshot func that will return expected snapshot after 3 invocations
		counter := 0
		getSnapshot := func(_ context.Context) (protocol.Snapshot, error) {
			if counter < 3 {
				counter++
				return oldSnapshot, nil
			}

			return expectedSnapshot, nil
		}

		_, _, targetPhase, targetEpoch := dynamicJoinFlagsFixture()

		// get snapshot
		actualSnapshot, err := common.GetSnapshotAtEpochAndPhase(
			context.Background(),
			unittest.Logger(),
			targetEpoch,
			targetPhase,
			time.Nanosecond,
			getSnapshot,
		)
		require.NoError(t, err)

		require.Equal(t, expectedSnapshot, actualSnapshot)
	})

	t.Run("should return snapshot immediately if target epoch has passed", func(t *testing.T) {
		// the snapshot that will return target counter and phase
		// epoch > target epoch but phase < target phase
		expectedSnapshot := getMockSnapshot(t, 5, flow.EpochPhaseStaking)

		// setup mock get snapshot func that will return expected snapshot after 3 invocations
		getSnapshot := func(_ context.Context) (protocol.Snapshot, error) {
			return expectedSnapshot, nil
		}

		_, _, targetPhase, targetEpoch := dynamicJoinFlagsFixture()

		// get snapshot
		actualSnapshot, err := common.GetSnapshotAtEpochAndPhase(
			context.Background(),
			unittest.Logger(),
			targetEpoch,
			targetPhase,
			time.Nanosecond,
			getSnapshot,
		)
		require.NoError(t, err)
		require.Equal(t, expectedSnapshot, actualSnapshot)
	})

	t.Run("should return snapshot after target phase is reached if target epoch is the same as current", func(t *testing.T) {
		oldSnapshot := getMockSnapshot(t, 5, flow.EpochPhaseStaking)
		expectedSnapshot := getMockSnapshot(t, 5, flow.EpochPhaseSetup)

		counter := 0
		// setup mock get snapshot func that will return expected snapshot after 3 invocations
		getSnapshot := func(_ context.Context) (protocol.Snapshot, error) {
			if counter < 3 {
				counter++
				return oldSnapshot, nil
			}

			return expectedSnapshot, nil
		}

		_, _, targetPhase, _ := dynamicJoinFlagsFixture()

		// get snapshot
		actualSnapshot, err := common.GetSnapshotAtEpochAndPhase(
			context.Background(),
			unittest.Logger(),
			5,
			targetPhase,
			time.Nanosecond,
			getSnapshot,
		)
		require.NoError(t, err)
		require.Equal(t, expectedSnapshot, actualSnapshot)
	})
}
