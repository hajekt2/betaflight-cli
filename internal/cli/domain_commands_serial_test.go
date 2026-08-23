package cli

import (
	"context"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestSerialApplyConfigV2JSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"ports":[{"identifier":20,"function_mask":1,"msp_baudrate_index":5,"gps_baudrate_index":0,"telemetry_baudrate_index":0,"blackbox_baudrate_index":5}]}`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-v2-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSerialApplyConfigV2JSONValidationBeforeConnect(t *testing.T) {
	input := `{"ports":[]}`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-v2-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}
