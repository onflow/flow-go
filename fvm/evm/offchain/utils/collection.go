package utils

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/onflow/flow-go-sdk/access/grpc"
)

func CollectEventData(conf *Config, path string) error {

	flowClient, err := grpc.NewClient(conf.host)
	if err != nil {
		return err
	}
	outputFile := filepath.Join(path, "events.jsonl")
	fi, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer fi.Close()

	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	ctx := context.Background()

	txEventType := fmt.Sprintf("A.%s.EVM.TransactionExecuted", conf.evmContractAddress)
	blockEventType := fmt.Sprintf("A.%s.EVM.BlockExecuted", conf.evmContractAddress)

	for height := conf.startHeight; height < conf.endHeight; height += conf.batchSize {
		events := make([]Event, 0)
		result, err := flowClient.GetEventsForHeightRange(ctx, txEventType, height, height+conf.batchSize-1)
		if err != nil {
			return err
		}
		if len(result) > 0 {
			for _, tEvent := range result {
				evs := tEvent.Events
				for _, e := range evs {
					events = append(events, Event{
						FlowBlockHeight: tEvent.Height,
						EventType:       e.Type,
						EventPayload:    hex.EncodeToString(e.Payload),
						txIndex:         e.TransactionIndex,
						eventIndex:      e.EventIndex,
					})
				}
			}
		}
		result, err = flowClient.GetEventsForHeightRange(ctx, blockEventType, height, height+conf.batchSize-1)
		if err != nil {
			return err
		}
		if len(result) > 0 {
			for _, bEvent := range result {
				evs := bEvent.Events
				for _, e := range evs {
					events = append(events, Event{
						FlowBlockHeight: bEvent.Height,
						EventType:       e.Type,
						EventPayload:    hex.EncodeToString(e.Payload),
						// setting to max int to make sure it is order as the last event of the evm block
						txIndex: math.MaxInt,
					})
				}
			}
		}

		// sort events by flow height, tx index and then event index
		sort.Slice(events, func(i, j int) bool {
			if events[i].FlowBlockHeight == events[j].FlowBlockHeight {
				if events[i].txIndex == events[j].txIndex {
					return events[i].eventIndex < events[j].eventIndex
				}
				return events[i].txIndex < events[j].txIndex
			}
			return events[i].FlowBlockHeight < events[j].FlowBlockHeight
		})

		for _, ev := range events {
			jsonData, err := json.Marshal(ev)
			if err != nil {
				return err
			}
			_, err = writer.WriteString(string(jsonData) + "\n")
			if err != nil {
				return err
			}
			err = writer.Flush()
			if err != nil {
				return err
			}
		}
	}
	return writer.Flush()
}
