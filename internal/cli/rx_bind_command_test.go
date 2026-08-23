package cli

import (
	"context"
	"encoding/json"
	"testing"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestRxBindStartReportsConfirmationRequiredWithoutConnect(t *testing.T) {
	connectCalls := 0
	connect := func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		connectCalls++
		return nil, connection.TargetInfo{}, nil
	}
	a := &app{connect: connect}
	cmd := a.rxBindCommand()
	cmd.SetArgs([]string{"start"})
	err := cmd.Execute()
	if !isExitError(err) {
		t.Fatalf("error = %v, want exit error for confirmation_required", err)
	}
	if connectCalls != 0 {
		t.Fatalf("connector invoked %d times without --yes", connectCalls)
	}
}

func TestRxBindResultEnvelopeSerialization(t *testing.T) {
	result := &bfcommands.RxBindResult{
		MSPCode:      msp.MSP2BetaflightBind,
		MSPName:      "MSP2_BETAFLIGHT_BIND",
		Acknowledged: true,
	}
	data := map[string]any{"rx_bind": result}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	bind := decoded["rx_bind"].(map[string]any)
	if bind["msp_code"] != float64(0x3000) || bind["msp_name"] != "MSP2_BETAFLIGHT_BIND" || bind["acknowledged"] != true {
		t.Fatalf("rx_bind = %+v", bind)
	}
}

func TestRxBindCommandStructure(t *testing.T) {
	a := &app{}
	cmd := a.rxBindCommand()
	if cmd.Use != "rx-bind" {
		t.Fatalf("Use = %q", cmd.Use)
	}
	subs := cmd.Commands()
	if len(subs) != 1 || subs[0].Use != "start" {
		t.Fatalf("subcommands = %+v", subs)
	}
}
