package collection

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/collection/ingest"
)

var _ commands.AdminCommand = (*TxRateLimitCommand)(nil)

// TxRateLimitCommand will adjust the transaction ingest rate limiter.
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
	addrList, err := ingest.ParseAddresses(addresses)
	if err != nil {
		return "", err
	}

	if command == "add" {
		ingest.AddAddresses(s.limiter, addrList)
		return fmt.Sprintf("added %d addresses", len(addrList)), nil
	}

	if command == "remove" {
		ingest.RemoveAddresses(s.limiter, addrList)
		return fmt.Sprintf("removed %d addresses", len(addrList)), nil
	}

	return "", fmt.Errorf("invalid command name %s (must be either 'add' or 'remove')", command)
}
