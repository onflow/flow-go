package request

import (
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/model/flow"
)

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

func (g *SubscribeEvents) Build(r *Request) error {
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
	var startBlockID ID
	err := startBlockID.Parse(rawStartBlockID)
	if err != nil {
		return err
	}
	g.StartBlockID = startBlockID.Flow()

	var height Height
	err = height.Parse(rawStartHeight)
	if err != nil {
		return fmt.Errorf("invalid start height: %w", err)
	}
	g.StartHeight = height.Flow()

	// if both start_block_id and start_height are provided
	if g.StartBlockID != flow.ZeroID && g.StartHeight != EmptyHeight {
		return fmt.Errorf("can only provide either block ID or start height")
	}

	// default to root block
	if g.StartHeight == EmptyHeight {
		g.StartHeight = 0
	}

	var eventTypes EventTypes
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
