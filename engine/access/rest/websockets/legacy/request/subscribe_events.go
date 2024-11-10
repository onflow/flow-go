package request

import (
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/convert"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
	"github.com/onflow/flow-go/model/flow"
)

const startHeightQuery = "start_height"
const startBlockIdQuery = "start_block_id"
const eventTypesQuery = "event_types"
const addressesQuery = "addresses"
const contractsQuery = "contracts"
const heartbeatIntervalQuery = "heartbeat_interval"

type SubscribeEvents struct {
	StartBlockID flow.Identifier
	StartHeight  uint64

	EventTypes []string
	Addresses  []string
	Contracts  []string

	HeartbeatInterval uint64
}

// SubscribeEventsRequest extracts necessary variables from the provided request,
// builds a SubscribeEvents instance, and validates it.
//
// No errors are expected during normal operation.
func SubscribeEventsRequest(r *common.Request) (SubscribeEvents, error) {
	var req SubscribeEvents
	err := req.Build(r)
	return req, err
}

func (g *SubscribeEvents) Build(r *common.Request) error {
	return g.Parse(
		r.GetQueryParam(startBlockIdQuery),
		r.GetQueryParam(startHeightQuery),
		r.GetQueryParams(eventTypesQuery),
		r.GetQueryParams(addressesQuery),
		r.GetQueryParams(contractsQuery),
		r.GetQueryParam(heartbeatIntervalQuery),
	)
}

func (g *SubscribeEvents) Parse(
	rawStartBlockID string,
	rawStartHeight string,
	rawTypes []string,
	rawAddresses []string,
	rawContracts []string,
	rawHeartbeatInterval string,
) error {
	var startBlockID convert.ID
	err := startBlockID.Parse(rawStartBlockID)
	if err != nil {
		return err
	}
	g.StartBlockID = startBlockID.Flow()

	var height request.Height
	err = height.Parse(rawStartHeight)
	if err != nil {
		return fmt.Errorf("invalid start height: %w", err)
	}
	g.StartHeight = height.Flow()

	// if both start_block_id and start_height are provided
	if g.StartBlockID != flow.ZeroID && g.StartHeight != request.EmptyHeight {
		return fmt.Errorf("can only provide either block ID or start height")
	}

	// default to root block
	if g.StartHeight == request.EmptyHeight {
		g.StartHeight = 0
	}

	var eventTypes request.EventTypes
	err = eventTypes.Parse(rawTypes)
	if err != nil {
		return err
	}

	g.EventTypes = eventTypes.Flow()
	g.Addresses = rawAddresses
	g.Contracts = rawContracts

	// parse heartbeat interval
	if rawHeartbeatInterval == "" {
		// set zero if the interval wasn't passed in request, so we can check it later and apply any default value if needed
		g.HeartbeatInterval = 0
		return nil
	}

	g.HeartbeatInterval, err = strconv.ParseUint(rawHeartbeatInterval, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid heartbeat interval format")
	}

	return nil
}
