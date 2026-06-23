package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type DataflashReadRequest struct {
	Address          uint32 `json:"address"`
	BlockSize        uint16 `json:"block_size"`
	AllowCompression bool   `json:"allow_compression"`
}

type DataflashReadChunk struct {
	Source          string `json:"source"`
	Address         uint32 `json:"address"`
	DataLength      uint16 `json:"data_length"`
	CompressionType uint8  `json:"compression_type"`
	Data            []byte `json:"-"`
}

func ReadDataflashChunk(ctx context.Context, client *connection.Client, req DataflashReadRequest) (*DataflashReadChunk, error) {
	payload := make([]byte, 0, 7)
	payload = append(payload,
		byte(req.Address),
		byte(req.Address>>8),
		byte(req.Address>>16),
		byte(req.Address>>24),
		byte(req.BlockSize),
		byte(req.BlockSize>>8),
	)
	if req.AllowCompression {
		payload = append(payload, 1)
	} else {
		payload = append(payload, 0)
	}
	frame, err := client.Request(ctx, msp.MSPDataflashRead, payload)
	if err != nil {
		return nil, fmt.Errorf("dataflash read failed at 0x%08x: %w", req.Address, err)
	}
	return DecodeDataflashReadChunk(frame.Payload)
}

func DecodeDataflashReadChunk(payload []byte) (*DataflashReadChunk, error) {
	const headerSize = 7
	if len(payload) < headerSize {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_DATAFLASH_READ header size %d", len(payload), headerSize)
	}
	r := msp.NewPayloadReader(payload)
	address, err := r.U32()
	if err != nil {
		return nil, err
	}
	dataLength, err := r.U16()
	if err != nil {
		return nil, err
	}
	compressionType, err := r.U8()
	if err != nil {
		return nil, err
	}
	if compressionType != 0 {
		return nil, fmt.Errorf("unsupported MSP_DATAFLASH_READ compression type %d", compressionType)
	}
	if r.Remaining() < int(dataLength) {
		return nil, fmt.Errorf("MSP_DATAFLASH_READ declared %d data bytes but only %d remain", dataLength, r.Remaining())
	}
	data := append([]byte(nil), payload[headerSize:headerSize+int(dataLength)]...)
	return &DataflashReadChunk{
		Source:          "MSP_DATAFLASH_READ",
		Address:         address,
		DataLength:      dataLength,
		CompressionType: compressionType,
		Data:            data,
	}, nil
}
