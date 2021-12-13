package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"net/url"
	"testing"
)

func statusReq(t *testing.T) *http.Request {
	u, _ := url.Parse("/v1/status")
	req, err := http.NewRequest("GET", u.String(), nil)
	require.NoError(t, err)

	return req
}

func TestStatusSuccess(t *testing.T) {
	backend := &mock.API{}
	req := statusReq(t)
	backend.Mock.
		On("Ping", mocks.Anything).
		Return(nil)

	backend.Mock.
		On("GetNetworkParameters", mocks.Anything).
		Return(access.NetworkParameters{
			ChainID: flow.Testnet,
		})

	assertOKResponse(t, req, `{"chain_id": "flow-testnet"}`, backend)
}

func TestStatusFailure(t *testing.T) {
	backend := &mock.API{}
	req := statusReq(t)
	backend.Mock.
		On("Ping", mocks.Anything).
		Return(status.Error(codes.Internal, "internal error"))

	assertResponse(t, req, http.StatusInternalServerError, `{"code":500,"message":"internal server error"}`, backend)
}
