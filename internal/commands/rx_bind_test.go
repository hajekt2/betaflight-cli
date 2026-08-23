package commands

import (
	"encoding/json"
	"testing"

	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestStartRxBindResultSerialization(t *testing.T) {
	result := &RxBindResult{
		MSPCode:      msp.MSP2BetaflightBind,
		MSPName:      "MSP2_BETAFLIGHT_BIND",
		Acknowledged: true,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["msp_code"] != float64(msp.MSP2BetaflightBind) {
		t.Fatalf("msp_code = %v, want %d", decoded["msp_code"], msp.MSP2BetaflightBind)
	}
	if decoded["msp_name"] != "MSP2_BETAFLIGHT_BIND" {
		t.Fatalf("msp_name = %v", decoded["msp_name"])
	}
	if decoded["acknowledged"] != true {
		t.Fatalf("acknowledged = %v", decoded["acknowledged"])
	}
}

func TestMSP2BetaflightBindConstant(t *testing.T) {
	if msp.MSP2BetaflightBind != 0x3000 {
		t.Fatalf("MSP2BetaflightBind = %#04x, want 0x3000", msp.MSP2BetaflightBind)
	}
	meta, ok := msp.LookupCommandByName("MSP2_BETAFLIGHT_BIND")
	if !ok || meta.Code != msp.MSP2BetaflightBind || meta.Protocol != 2 {
		t.Fatalf("metadata = %+v ok = %v", meta, ok)
	}
}
