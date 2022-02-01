package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

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
	t.Run("should log 4 messages, 3 warnings and 1 info before returning successfully", func(t *testing.T) {
		// the snapshot we will use to force GetSnapshotAtEpochAndPhase to retry
		snapshot := getMockSnapshot(t, 0, flow.EpochPhaseStaking)

		// the snapshot that will return target counter and phase
		expectedSnapshot := getMockSnapshot(t, 1, flow.EpochPhaseSetup)

		// setup hooked logger to capture warn and info log counts
		hookWarnCalls := 0
		hookInfoCalls := 0
		hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			if level == zerolog.WarnLevel {
				hookWarnCalls++
			} else if level == zerolog.InfoLevel {
				hookInfoCalls++
			}
		})

		logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).Hook(hook)

		// setup mock get snapshot func that will return expected snapshot after 3 invocations
		counter := 0
		getSnapshot := func() (protocol.Snapshot, error) {
			if counter < 3 {
				counter++
				return snapshot, nil
			}

			return expectedSnapshot, nil
		}

		_, _, targetPhase, targetEpoch := dynamicJoinFlagsFixture()

		// get snapshot
		actualSnapshot, err := GetSnapshotAtEpochAndPhase(
			logger,
			targetEpoch,
			targetPhase,
			time.Millisecond,
			getSnapshot,
		)
		require.NoError(t, err)

		require.Equalf(t, 3, hookWarnCalls, "expected 3 warn logs got %d", hookWarnCalls)
		require.Equalf(t, 1, hookInfoCalls, "expected 1 info log got %d", hookInfoCalls)
		require.Equal(t, expectedSnapshot, actualSnapshot)
	})
	t.Run("should return snapshot immediately if target epoch has passed", func(t *testing.T) {
		// the snapshot that will return target counter and phase
		// epoch > target epoch but phase < target phase
		expectedSnapshot := getMockSnapshot(t, 5, flow.EpochPhaseStaking)

		// setup hooked logger to capture warn and info log counts
		hookWarnCalls := 0
		hookInfoCalls := 0
		hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			if level == zerolog.WarnLevel {
				hookWarnCalls++
			} else if level == zerolog.InfoLevel {
				hookInfoCalls++
			}
		})

		logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).Hook(hook)

		// setup mock get snapshot func that will return expected snapshot after 3 invocations
		getSnapshot := func() (protocol.Snapshot, error) {
			return expectedSnapshot, nil
		}

		_, _, targetPhase, targetEpoch := dynamicJoinFlagsFixture()

		// get snapshot
		actualSnapshot, err := GetSnapshotAtEpochAndPhase(
			logger,
			targetEpoch,
			targetPhase,
			time.Millisecond,
			getSnapshot,
		)
		require.NoError(t, err)

		require.Equalf(t, 0, hookWarnCalls, "expected 0 warn logs got %d", hookWarnCalls)
		require.Equalf(t, 1, hookInfoCalls, "expected 1 info log got %d", hookInfoCalls)
		require.Equal(t, expectedSnapshot, actualSnapshot)
	})
	t.Run("should return snapshot after target phase is reached if target epoch is the same as current", func(t *testing.T) {
		snapshot := getMockSnapshot(t, 5, flow.EpochPhaseStaking)
		expectedSnapshot := getMockSnapshot(t, 5, flow.EpochPhaseSetup)

		// setup hooked logger to capture warn and info log counts
		hookWarnCalls := 0
		hookInfoCalls := 0
		hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			if level == zerolog.WarnLevel {
				hookWarnCalls++
			} else if level == zerolog.InfoLevel {
				hookInfoCalls++
			}
		})

		logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).Hook(hook)

		counter := 0
		// setup mock get snapshot func that will return expected snapshot after 3 invocations
		getSnapshot := func() (protocol.Snapshot, error) {
			if counter < 3 {
				counter++
				return snapshot, nil
			}

			return expectedSnapshot, nil
		}

		_, _, targetPhase, _ := dynamicJoinFlagsFixture()

		// get snapshot
		actualSnapshot, err := GetSnapshotAtEpochAndPhase(
			logger,
			5,
			targetPhase,
			time.Millisecond,
			getSnapshot,
		)
		require.NoError(t, err)

		require.Equalf(t, 3, hookWarnCalls, "expected 0 warn logs got %d", hookWarnCalls)
		require.Equalf(t, 1, hookInfoCalls, "expected 1 info log got %d", hookInfoCalls)
		require.Equal(t, expectedSnapshot, actualSnapshot)
	})
}
