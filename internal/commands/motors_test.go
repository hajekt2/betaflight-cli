package commands

import (
	"bytes"
	"testing"
)

func TestDecodeMotorConfigAndOutputs(t *testing.T) {
	payload := appendU16Test(nil, 0)
	payload = appendU16Test(payload, 2000)
	payload = appendU16Test(payload, 1000)
	payload = append(payload, 4, 14, 1, 1)
	config, err := DecodeMotorConfig(payload)
	if err != nil {
		t.Fatalf("DecodeMotorConfig() error = %v", err)
	}
	if config.MaxThrottle != 2000 || config.MinCommand != 1000 {
		t.Fatalf("config = %+v", config)
	}
	if config.MotorCount == nil || *config.MotorCount != 4 || config.UseDShotTelemetry == nil || !*config.UseDShotTelemetry {
		t.Fatalf("config = %+v", config)
	}
	outputs, err := DecodeU16Array([]byte{0xe8, 0x03, 0xe9, 0x03})
	if err != nil {
		t.Fatalf("DecodeU16Array() error = %v", err)
	}
	if len(outputs) != 2 || outputs[0] != 1000 || outputs[1] != 1001 {
		t.Fatalf("outputs = %+v", outputs)
	}
}

func TestDecodeMotorTelemetry(t *testing.T) {
	payload := []byte{1}
	payload = appendU32Test(payload, 12500)
	payload = appendU16Test(payload, 25)
	payload = append(payload, 42)
	payload = appendU16Test(payload, 1680)
	payload = appendU16Test(payload, 230)
	payload = appendU16Test(payload, 120)
	telemetry, err := DecodeMotorTelemetry(payload)
	if err != nil {
		t.Fatalf("DecodeMotorTelemetry() error = %v", err)
	}
	if len(telemetry) != 1 || telemetry[0].RPM != 12500 || telemetry[0].TemperatureC != 42 {
		t.Fatalf("telemetry = %+v", telemetry)
	}
	if telemetry[0].InvalidPercent != 0.25 || telemetry[0].VoltageV != 16.8 || telemetry[0].CurrentA != 2.3 {
		t.Fatalf("telemetry = %+v", telemetry)
	}
}

func TestDecodeMotor3DAndOutputOrder(t *testing.T) {
	config, err := DecodeMotor3DConfig([]byte{0x7e, 0x05, 0xea, 0x05, 0xb4, 0x05})
	if err != nil {
		t.Fatalf("DecodeMotor3DConfig() error = %v", err)
	}
	if config.DeadbandLow != 1406 || config.DeadbandHigh != 1514 || config.Neutral != 1460 {
		t.Fatalf("config = %+v", config)
	}
	payload := EncodeMotor3DConfig(Motor3DConfig{DeadbandLow: 1406, DeadbandHigh: 1514, Neutral: 1460})
	if !bytes.Equal(payload, []byte{0x7e, 0x05, 0xea, 0x05, 0xb4, 0x05}) {
		t.Fatalf("payload = %v", payload)
	}
	order, err := DecodeMotorOutputOrder([]byte{4, 0, 1, 2, 3})
	if err != nil {
		t.Fatalf("DecodeMotorOutputOrder() error = %v", err)
	}
	if len(order) != 4 || order[3] != 3 {
		t.Fatalf("order = %+v", order)
	}
}

func TestDecodeServoConfigurationAndRules(t *testing.T) {
	payload := appendU16Test(nil, 1000)
	payload = appendU16Test(payload, 2000)
	payload = appendU16Test(payload, 1500)
	payload = append(payload, byte(100), 255)
	payload = appendU32Test(payload, 0x05)
	configs, err := DecodeServoConfigurations(payload)
	if err != nil {
		t.Fatalf("DecodeServoConfigurations() error = %v", err)
	}
	if len(configs) != 1 || configs[0].Rate != 100 || configs[0].ForwardFromChannel != 255 || configs[0].ReversedSourcesMask != 5 {
		t.Fatalf("configs = %+v", configs)
	}
	rules, err := DecodeServoMixRules([]byte{0, 1, 100, 10, 0, 100, 2, 0, 0, 0, 0, 0, 0, 0})
	if err != nil {
		t.Fatalf("DecodeServoMixRules() error = %v", err)
	}
	if len(rules) != 2 || !rules[0].Active || rules[1].Active || rules[0].Rate != 100 {
		t.Fatalf("rules = %+v", rules)
	}
}
