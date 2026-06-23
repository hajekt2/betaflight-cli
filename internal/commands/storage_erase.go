package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type DataflashEraseResult struct {
	Source string `json:"source"`
	Code   uint16 `json:"code"`
}

func EraseDataflash(ctx context.Context, client *connection.Client) (*DataflashEraseResult, error) {
	if _, err := client.Request(ctx, msp.MSPDataflashErase, nil); err != nil {
		return nil, fmt.Errorf("dataflash erase failed: %w", err)
	}
	return &DataflashEraseResult{
		Source: "MSP_DATAFLASH_ERASE",
		Code:   msp.MSPDataflashErase,
	}, nil
}
