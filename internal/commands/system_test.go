package commands

import "testing"

func TestParseSystemStatus(t *testing.T) {
	status := ParseSystemStatus([]string{
		"CONFIG: CONFIGURED (3820b / 16384b)",
		"DEVICES DETECTED: SPI=2, I2C=1 (0 errors)",
		"MCU: STM32F40X CLK=168MHz (PLLP-HSE), Vref=3.26V, Core temp=42degC",
		"STACK: 2048b (0x1000fff0)",
		"GYRO: (1) BMI270 enabled locked dma",
		"ACC: ICM42688P",
		"GPS: connected, UART1 115200 (set to AUTO), configured, version =  M10",
		"OSD: MSP (53 x 20)",
		"FLASH: JEDEC ID=0x00abcdef 16M",
		"BUILD KEY: fake-build-key (2025.12.1)",
		"System Uptime: 123 seconds, Current Time: 2026-06-23 12:34:56",
		"CPU:42%, cycle time: 250, GYRO rate: 4000, RX rate: 250, System rate: 10",
		"Voltage: 15.99V (4S battery - OK)",
		"Arming disable flags: RXLOSS THROTTLE",
	})
	if status.Config == nil || status.Config.State != "CONFIGURED" || status.Config.UsedBytes != 3820 || status.Config.MaxBytes != 16384 {
		t.Fatalf("config = %+v", status.Config)
	}
	if status.Devices == nil || status.Devices.SPI == nil || *status.Devices.SPI != 2 || status.Devices.I2CErrors == nil || *status.Devices.I2CErrors != 0 {
		t.Fatalf("devices = %+v", status.Devices)
	}
	if status.MCU == nil || status.MCU.Name != "STM32F40X" || status.MCU.ClockMHz == nil || *status.MCU.ClockMHz != 168 || status.MCU.ClockSource != "PLLP-HSE" {
		t.Fatalf("mcu = %+v", status.MCU)
	}
	if status.MCU.Vref == nil || *status.MCU.Vref != 3.26 || status.MCU.CoreTemperatureC == nil || *status.MCU.CoreTemperatureC != 42 {
		t.Fatalf("mcu analog = %+v", status.MCU)
	}
	if status.Stack == nil || status.Stack.Bytes != 2048 || status.Stack.TopHex != "0x1000fff0" {
		t.Fatalf("stack = %+v", status.Stack)
	}
	if status.ACCLine != "ACC: ICM42688P" {
		t.Fatalf("acc = %q", status.ACCLine)
	}
	if status.BuildKey == nil || status.BuildKey.Key != "fake-build-key" || status.BuildKey.Release != "2025.12.1" {
		t.Fatalf("build key = %+v", status.BuildKey)
	}
	if status.Uptime == nil || status.Uptime.Seconds != 123 || status.Uptime.CurrentTime != "2026-06-23 12:34:56" {
		t.Fatalf("uptime = %+v", status.Uptime)
	}
	if status.Runtime == nil || status.Runtime.CPULoadPercent != 42 || status.Runtime.GyroRateHz != 4000 || status.Runtime.SystemRateHz != 10 {
		t.Fatalf("runtime = %+v", status.Runtime)
	}
	if status.Voltage == nil || status.Voltage.VoltageV != 15.99 || status.Voltage.CellCount != 4 || status.Voltage.BatteryState != "OK" {
		t.Fatalf("voltage = %+v", status.Voltage)
	}
	if status.Arming == nil || len(status.Arming.Flags) != 2 || status.Arming.Flags[0] != "RXLOSS" {
		t.Fatalf("arming = %+v", status.Arming)
	}
	if len(status.Unparsed) != 0 {
		t.Fatalf("unparsed = %+v", status.Unparsed)
	}
}
