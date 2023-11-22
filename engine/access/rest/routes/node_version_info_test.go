package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/utils/unittest"
)

func nodeVersionInfoURL(t *testing.T) string {
	u, err := url.ParseRequestURI("/v1/node_version_info")
	require.NoError(t, err)

	return u.String()
}

func TestGetNodeVersionInfo(t *testing.T) {
	backend := mock.NewAPI(t)

	t.Run("get node version info", func(t *testing.T) {
		req := getNodeVersionInfoRequest(t)

		params := &access.NodeVersionInfo{
			Semver:               build.Version(),
			Commit:               build.Commit(),
			SporkId:              unittest.IdentifierFixture(),
			ProtocolVersion:      unittest.Uint64InRange(10, 30),
			SporkRootBlockHeight: unittest.Uint64InRange(1000, 10_000),
			NodeRootBlockHeight:  unittest.Uint64InRange(10_000, 100_000),
		}

		backend.Mock.
			On("GetNodeVersionInfo", mocktestify.Anything).
			Return(params, nil)

		expected := nodeVersionInfoExpectedStr(params)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})
}

func nodeVersionInfoExpectedStr(nodeVersionInfo *access.NodeVersionInfo) string {
	return fmt.Sprintf(`{
			"semver": "%s",
			"commit": "%s",
			"spork_id": "%s",
            "protocol_version": "%d",
            "spork_root_block_height": "%d",
            "node_root_block_height": "%d"
		}`,
		nodeVersionInfo.Semver,
		nodeVersionInfo.Commit,
		nodeVersionInfo.SporkId.String(),
		nodeVersionInfo.ProtocolVersion,
		nodeVersionInfo.SporkRootBlockHeight,
		nodeVersionInfo.NodeRootBlockHeight,
	)
}

func getNodeVersionInfoRequest(t *testing.T) *http.Request {
	req, err := http.NewRequest("GET", nodeVersionInfoURL(t), nil)
	require.NoError(t, err)
	return req
}
