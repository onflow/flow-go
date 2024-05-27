package version

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	testifyMock "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type versionControlMockHeaders struct {
	headers map[uint64]*flow.Header
}

func (m *versionControlMockHeaders) BlockIDByHeight(height uint64) (flow.Identifier, error) {
	h, ok := m.headers[height]
	if !ok {
		return flow.ZeroID, fmt.Errorf("header not found")
	}
	return h.ID(), nil
}

var _ VersionControlHeaders = (*versionControlMockHeaders)(nil)

func TestInitialization(t *testing.T) {

	t.Run("happy case", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		versionBeacons := new(storageMock.VersionBeacons)

		sealedHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(10))

		newBlockHeader := unittest.BlockHeaderWithParentFixture(sealedHeader) // 11

		headers := &versionControlMockHeaders{
			headers: map[uint64]*flow.Header{
				sealedHeader.Height: sealedHeader,
			},
		}

		vc := NewVersionControl(
			unittest.Logger(),
			headers,
			versionBeacons,
			semver.New("0.0.1"),
			sealedHeader,
		)

		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						flow.VersionBoundary{
							BlockHeight: sealedHeader.Height,
							Version:     "0.0.1",
						}),
				),
				SealHeight: sealedHeader.Height,
			}, nil)

		ictx := irrecoverable.NewMockSignalerContext(t, ctx)
		vc.Start(ictx)

		unittest.AssertClosesBefore(t, vc.Ready(), 10*time.Second)

		assert.True(t, vc.CompatibleAtBlock(sealedHeader.Height))
		assert.False(t, vc.CompatibleAtBlock(newBlockHeader.Height))
	})

	t.Run("no version beacon found", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		versionBeacons := new(storageMock.VersionBeacons)

		sealedHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(19))

		newBlockHeader := unittest.BlockHeaderWithParentFixture(sealedHeader) // 20

		headers := &versionControlMockHeaders{
			headers: map[uint64]*flow.Header{
				sealedHeader.Height: sealedHeader,
			},
		}

		vc := NewVersionControl(
			unittest.Logger(),
			headers,
			versionBeacons,
			semver.New("0.0.1"),
			sealedHeader,
		)

		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(nil, nil)

		ictx := irrecoverable.NewMockSignalerContext(t, ctx)
		vc.Start(ictx)

		unittest.AssertClosesBefore(t, vc.Ready(), 10*time.Second)

		assert.False(t, vc.CompatibleAtBlock(sealedHeader.Height))
		assert.False(t, vc.CompatibleAtBlock(newBlockHeader.Height))
	})
}
