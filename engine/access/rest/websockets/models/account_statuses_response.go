package models

import (
	"strconv"

	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

// Build creates AccountStatusesResponse instance.
func (e *AccountStatusesResponse) Build(accountStatusesResponse *backend.AccountStatusesResponse, index uint64) {
	var accountEvents AccountEvents
	accountEvents.Build(accountStatusesResponse.AccountEvents)

	*e = AccountStatusesResponse{
		BlockID:       accountStatusesResponse.BlockID.String(),
		Height:        strconv.FormatUint(accountStatusesResponse.Height, 10),
		AccountEvents: accountEvents,
		MessageIndex:  index,
	}
}
