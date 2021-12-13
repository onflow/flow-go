package rest

import "github.com/onflow/flow-go/access"

// getStatus of the network.
func getStatus(r *request, backend access.API, _ LinkGenerator) (interface{}, error) {
	err := backend.Ping(r.Context())
	if err != nil {
		return nil, err
	}

	network := backend.GetNetworkParameters(r.Context())
	return networkStatusResponse(network), nil
}
