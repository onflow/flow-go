package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

const eventTypeQuery = "type"
const blockIdsQuery = "block_ids"
const MaxEventRequestHeightRange = 250
const agreeingExecutorCountQuery = "agreeing_executors_count"
const requiredExecutorIdsQuery = "required_executor_ids"
const includeExecutorMetadataQuery = "include_executor_metadata"

type GetEvents struct {
	StartHeight    uint64
	EndHeight      uint64
	Type           string
	BlockIDs       []flow.Identifier
	ExecutionState models.ExecutionStateQuery
}

func NewGetEvents(r *common.Request) (GetEvents, error) {
	return parseGetEvents(
		r.GetQueryParam(eventTypeQuery),
		r.GetQueryParam(startHeightQuery),
		r.GetQueryParam(endHeightQuery),
		r.GetQueryParams(blockIdsQuery),
		r.GetQueryParam(agreeingExecutorCountQuery),
		r.GetQueryParams(requiredExecutorIdsQuery),
		r.GetQueryParam(includeExecutorMetadataQuery),
	)
}

// parseGetEvents validates given request parameters and returns a GetEvents struct.
//
// No errors are expected during normal operation.
func parseGetEvents(
	rawEventType string,
	rawStartHeight string,
	rawEndHeight string,
	rawBlockIds []string,
	rawAgreeingExecutorsCount string,
	rawAgreeingExecutorsIds []string,
	rawIncludeExecutorMetadata string,
) (GetEvents, error) {
	var startHeight Height
	err := startHeight.Parse(rawStartHeight)
	if err != nil {
		return GetEvents{}, fmt.Errorf("invalid start height: %w", err)
	}

	var endHeight Height
	err = endHeight.Parse(rawEndHeight)
	if err != nil {
		return GetEvents{}, fmt.Errorf("invalid end height: %w", err)
	}

	blockIDs, err := parser.NewIDs(rawBlockIds)
	if err != nil {
		return GetEvents{}, err
	}

	isStartHeightProvided := startHeight.Flow() != EmptyHeight
	isEndHeightProvided := endHeight.Flow() != EmptyHeight
	isBlockIDsProvided := len(blockIDs) > 0

	// if both block IDs and (start or end height) are provided
	if isBlockIDsProvided && (isStartHeightProvided || isEndHeightProvided) {
		return GetEvents{}, fmt.Errorf("can only provide either block IDs or start and end height range")
	}

	// if neither block IDs nor both heights are provided
	if !isBlockIDsProvided && (!isStartHeightProvided || !isEndHeightProvided) {
		return GetEvents{}, fmt.Errorf("must provide either block IDs or start and end height range")
	}

	if rawEventType == "" {
		return GetEvents{}, fmt.Errorf("event type must be provided")
	}

	eventType, err := parser.NewEventType(rawEventType)
	if err != nil {
		return GetEvents{}, err
	}

	// validate start end height option
	if isStartHeightProvided && isEndHeightProvided {
		if startHeight > endHeight {
			return GetEvents{}, fmt.Errorf("start height must be less than or equal to end height")
		}

		isEndHeightFinalOrSealed := endHeight.Flow() == FinalHeight || endHeight.Flow() == SealedHeight
		isRangeTooLarge := endHeight-startHeight >= MaxEventRequestHeightRange

		if isRangeTooLarge && !isEndHeightFinalOrSealed {
			return GetEvents{}, fmt.Errorf(
				"height range %d exceeds maximum allowed of %d",
				endHeight-startHeight,
				MaxEventRequestHeightRange,
			)
		}
	}

	executionStateQuery, err := parser.NewExecutionStateQuery(
		rawAgreeingExecutorsCount,
		rawAgreeingExecutorsIds,
		rawIncludeExecutorMetadata,
	)
	if err != nil {
		return GetEvents{}, err
	}

	return GetEvents{
		StartHeight:    startHeight.Flow(),
		EndHeight:      endHeight.Flow(),
		Type:           eventType.Flow(),
		BlockIDs:       blockIDs.Flow(),
		ExecutionState: *executionStateQuery,
	}, nil
}
