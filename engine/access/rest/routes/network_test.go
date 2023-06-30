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
	"github.com/onflow/flow-go/model/flow"
)

func networkURL(t *testing.T) string {
	u, err := url.ParseRequestURI("/v1/network/parameters")
	require.NoError(t, err)

	return u.String()
}

func TestGetNetworkParameters(t *testing.T) {
	backend := &mock.API{}

	t.Run("get network parameters on mainnet", func(t *testing.T) {

		req := getNetworkParametersRequest(t)

		params := access.NetworkParameters{
			ChainID: flow.Mainnet,
		}

		backend.Mock.
			On("GetNetworkParameters", mocktestify.Anything).
			Return(params)

		expected := networkParametersExpectedStr(flow.Mainnet)

		assertOKResponse(t, req, expected, backend)
		mocktestify.AssertExpectationsForObjects(t, backend)
	})
}

func networkParametersExpectedStr(chainID flow.ChainID) string {
	return fmt.Sprintf(`{
			  "chain_id": "%s"
			}`, chainID)
}

func getNetworkParametersRequest(t *testing.T) *http.Request {
	req, err := http.NewRequest("GET", networkURL(t), nil)
	require.NoError(t, err)
	return req
}
