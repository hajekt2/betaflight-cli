package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

// RxBindResult reports the outcome of an MSP2_BETAFLIGHT_BIND request.
type RxBindResult struct {
	MSPCode      uint16 `json:"msp_code"`
	MSPName      string `json:"msp_name"`
	Acknowledged bool   `json:"acknowledged"`
}

// StartRxBind triggers receiver bind pulses over MSP.
//
// Upstream Betaflight 2026.6.1 handles MSP2_BETAFLIGHT_BIND in
// src/main/msp/msp.c (case at line 4334) by calling startRxBind() and
// returning MSP_RESULT_ERROR when binding cannot be started; the request
// carries no payload bytes. startRxBind() (src/main/rx/rx_bind.c:111) emits
// bind pulses for SPI RX protocols (FrSky SFHSS/CC2500 variants, FlySky,
// Spektrum, ExpressLRS) and, when compiled in, serial protocols such as
// CRSF-SRXL2 that support software binding.
func StartRxBind(ctx context.Context, client *connection.Client) (*RxBindResult, error) {
	if _, err := client.Request(ctx, msp.MSP2BetaflightBind, nil); err != nil {
		return nil, fmt.Errorf("rx bind request failed: %w", err)
	}
	return &RxBindResult{
		MSPCode:      msp.MSP2BetaflightBind,
		MSPName:      "MSP2_BETAFLIGHT_BIND",
		Acknowledged: true,
	}, nil
}
