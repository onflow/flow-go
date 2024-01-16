package collection

import (
	"context"
	"fmt"
	"strings"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/onflow/flow-go/model/flow"
)

var _ commands.AdminCommand = (*TxRateLimitCommand)(nil)

// TxRateLimitCommand will send a signal to compactor to trigger checkpoint
// once finishing writing the current WAL segment file
type TxRateLimitCommand struct {
	limiter *ingest.AddressRateLimiter
}

func NewTxRateLimitCommand(limiter *ingest.AddressRateLimiter) *TxRateLimitCommand {
	return &TxRateLimitCommand{
		limiter: limiter,
	}
}

func (s *TxRateLimitCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	input, ok := req.Data.(map[string]string)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected { \"command\": \"add|remove|get\", \"addresses\": \"addresses\""), nil
	}

	cmd, ok := input["command"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("the \"command\" field is empty, must be either \"add\" or \"remove\" or \"get\""), nil
	}

	if cmd == "get" {
		return s.limiter.GetAddresses(), nil
	}

	addresses, ok := input["addresses"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("the \"addresses\" field is empty, must be hex formated addresses, can be splitted by \",\""), nil
	}

	resp, err := s.Handle(cmd, addresses)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *TxRateLimitCommand) Validator(_ *admin.CommandRequest) error {
	return nil
}

func (s *TxRateLimitCommand) Handle(command string, addresses string) (string, error) {
	addrList, err := splitAndTrim(addresses)
	if err != nil {
		return "", err
	}

	if command == "add" {
		for _, addr := range addrList {
			s.limiter.AddAddress(addr)
		}
		return fmt.Sprintf("added %d addresses", len(addrList)), nil
	}

	if command == "remove" {
		for _, addr := range addrList {
			s.limiter.RemoveAddress(addr)
		}
		return fmt.Sprintf("removed %d addresses", len(addrList)), nil
	}

	return "", fmt.Errorf("invalid command name %s (must be either 'add' or 'remove')", command)
}

// parse addresses string into a list of flow addresses
func splitAndTrim(addresses string) ([]flow.Address, error) {
	addressList := make([]flow.Address, 0)
	for _, addr := range strings.Split(addresses, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		flowAddr, err := flow.StringToAddress(addr)
		if err != nil {
			return nil, err
		}
		addressList = append(addressList, flowAddr)
	}
	return addressList, nil
}
