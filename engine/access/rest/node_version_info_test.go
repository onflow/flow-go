package rest

import (
	"fmt"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/utils/unittest"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/mock"
)

func nodeVersionInfoURL(t *testing.T) string {
	u, err := url.ParseRequestURI("/v1/network/node_version_info")
	require.NoError(t, err)

	return u.String()
}

func TestGetNodeVersionInfo(t *testing.T) {
	backend := &mock.API{}

	t.Run("get node version info", func(t *testing.T) {
		req := getNodeVersionInfoRequest(t)

		params := &access.NodeVersionInfo{
			Semver:          build.Semver(),
			Commit:          build.Commit(),
			SporkId:         unittest.IdentifierFixture(),
			ProtocolVersion: uint(unittest.Uint64InRange(10, 30)),
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
            "protocol_version": "%s"
		}`, nodeVersionInfo.Semver, nodeVersionInfo.Commit, nodeVersionInfo.SporkId.String(), strconv.FormatUint(uint64(nodeVersionInfo.ProtocolVersion), 10))
}

func getNodeVersionInfoRequest(t *testing.T) *http.Request {
	req, err := http.NewRequest("GET", nodeVersionInfoURL(t), nil)
	require.NoError(t, err)
	return req
}
