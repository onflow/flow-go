package collection

import (
	"context"
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/collection/ingest"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
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
		return admin.NewInvalidAdminReqFormatError("expected { \"command\": \"add|remove|get|get_config|set_config\", \"addresses\": \"addresses\""), nil
	}

	cmd, ok := input["command"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("the \"command\" field is empty, must be one of add|remove|get|get_config|set_config"), nil
	}

	if cmd == "get" {
		list := s.limiter.GetAddresses()
		return fmt.Sprintf("rate limited list contains a total of %d addresses: %v", len(list), list), nil
	}

	if cmd == "add" || cmd == "remove" {
		addresses, ok := input["addresses"]
		if !ok {
			return admin.NewInvalidAdminReqErrorf("the \"addresses\" field is empty, must be hex formated addresses, can be splitted by \",\""), nil
		}

		log.Info().Msgf("admintool %v addresses: %v", cmd, addresses)

		resp, err := s.AddOrRemove(cmd, addresses)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	if cmd == "get_config" {
		limit, burst := s.limiter.GetLimitConfig()
		return fmt.Sprintf("limit: %v, burst: %v", limit, burst), nil
	}

	if cmd == "set_config" {
		dataLimit, limit_ok := input["limit"]
		dataBurst, burst_ok := input["burst"]
		if !limit_ok || !burst_ok {
			return admin.NewInvalidAdminReqErrorf("the \"limit\" or \"burst\" field is empty, must be number"), nil
		}
		limit, err := strconv.ParseFloat(dataLimit, 64)
		if err != nil {
			return admin.NewInvalidAdminReqErrorf("the \"limit\" field is not number %v, %s", dataLimit, err), nil
		}

		burst, err := strconv.Atoi(dataBurst)
		if err != nil {
			return admin.NewInvalidAdminReqErrorf("the \"burst\" field is not number: %v, %s", dataBurst, err), nil
		}

		oldLimit, oldBurst := s.limiter.GetLimitConfig()
		log.Info().Msgf("admintool set_config limit: %v, burst: %v, old limit: %v, old burst: %v", limit, burst, oldLimit, oldBurst)
		s.limiter.SetLimitConfig(rate.Limit(limit), burst)
		return fmt.Sprintf("succesfully set limit %v, burst %v", limit, burst), nil
	}

	return fmt.Sprintf(
		"invalid command field (%s), must be either \"add\" or \"remove\" or \"get\" or \"get_config\" or \"set_config\"",
		cmd), nil
}

func (s *TxRateLimitCommand) Validator(_ *admin.CommandRequest) error {
	return nil
}

func (s *TxRateLimitCommand) AddOrRemove(command string, addresses string) (string, error) {
	addrList, err := ingest.ParseAddresses(addresses)
	if err != nil {
		return "", err
	}

	if command == "add" {
		ingest.AddAddresses(s.limiter, addrList)
		return fmt.Sprintf("added %d addresses", len(addrList)), nil
	}

	// command == "remove"
	ingest.RemoveAddresses(s.limiter, addrList)
	return fmt.Sprintf("removed %d addresses", len(addrList)), nil
}
