package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type EnvironmentStatus struct {
	Altitude    *AltitudeReading    `json:"altitude,omitempty"`
	Rangefinder *RangefinderReading `json:"rangefinder,omitempty"`
	Analog      *AnalogReading      `json:"analog,omitempty"`
	Sources     map[string]string   `json:"sources,omitempty"`
}

type AltitudeReading struct {
	AltitudeCm int32   `json:"altitude_cm"`
	AltitudeM  float64 `json:"altitude_m"`
	VarioCmS   int16   `json:"vario_cm_s"`
}

type RangefinderReading struct {
	AltitudeCm uint32  `json:"altitude_cm"`
	AltitudeM  float64 `json:"altitude_m"`
}

type AnalogReading struct {
	VoltageLegacyV float64 `json:"voltage_legacy_v"`
	DrawnMAh       uint16  `json:"drawn_mah"`
	RSSI           uint16  `json:"rssi"`
	AmperageA      float64 `json:"amperage_a"`
	VoltageV       float64 `json:"voltage_v"`
}

func ReadEnvironmentStatus(ctx context.Context, client *connection.Client) (*EnvironmentStatus, []string, error) {
	status := &EnvironmentStatus{Sources: map[string]string{}}
	warnings := []string{}
	if frame, err := client.Request(ctx, msp.MSPAltitude, nil); err == nil {
		reading, err := DecodeAltitude(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_ALTITUDE decode failed: %v", err))
		} else {
			status.Altitude = reading
			status.Sources["altitude"] = "MSP_ALTITUDE"
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_ALTITUDE unavailable: %v", err))
	}
	if frame, err := client.Request(ctx, msp.MSPSonarAltitude, nil); err == nil {
		reading, err := DecodeRangefinderAltitude(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_SONAR_ALTITUDE decode failed: %v", err))
		} else {
			status.Rangefinder = reading
			status.Sources["rangefinder"] = "MSP_SONAR_ALTITUDE"
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_SONAR_ALTITUDE unavailable: %v", err))
	}
	if frame, err := client.Request(ctx, msp.MSPAnalog, nil); err == nil {
		reading, err := DecodeAnalog(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_ANALOG decode failed: %v", err))
		} else {
			status.Analog = reading
			status.Sources["analog"] = "MSP_ANALOG"
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_ANALOG unavailable: %v", err))
	}
	if status.Altitude == nil && status.Rangefinder == nil && status.Analog == nil {
		return nil, warnings, fmt.Errorf("environment status unavailable")
	}
	return status, warnings, nil
}

func DecodeAltitude(payload []byte) (*AltitudeReading, error) {
	const length = 6
	if len(payload) < length {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_ALTITUDE size %d", len(payload), length)
	}
	r := msp.NewPayloadReader(payload)
	altitudeRaw, err := r.U32()
	if err != nil {
		return nil, err
	}
	varioRaw, err := r.U16()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_ALTITUDE returned %d trailing byte(s)", r.Remaining())
	}
	altitude := int32(altitudeRaw)
	return &AltitudeReading{
		AltitudeCm: altitude,
		AltitudeM:  float64(altitude) / 100,
		VarioCmS:   int16(varioRaw),
	}, nil
}

func DecodeRangefinderAltitude(payload []byte) (*RangefinderReading, error) {
	const length = 4
	if len(payload) < length {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_SONAR_ALTITUDE size %d", len(payload), length)
	}
	r := msp.NewPayloadReader(payload)
	altitude, err := r.U32()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_SONAR_ALTITUDE returned %d trailing byte(s)", r.Remaining())
	}
	return &RangefinderReading{
		AltitudeCm: altitude,
		AltitudeM:  float64(altitude) / 100,
	}, nil
}

func DecodeAnalog(payload []byte) (*AnalogReading, error) {
	const length = 9
	if len(payload) < length {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_ANALOG size %d", len(payload), length)
	}
	r := msp.NewPayloadReader(payload)
	legacyVoltage, err := r.U8()
	if err != nil {
		return nil, err
	}
	drawn, err := r.U16()
	if err != nil {
		return nil, err
	}
	rssi, err := r.U16()
	if err != nil {
		return nil, err
	}
	amperage, err := r.S16()
	if err != nil {
		return nil, err
	}
	voltage, err := r.U16()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_ANALOG returned %d trailing byte(s)", r.Remaining())
	}
	return &AnalogReading{
		VoltageLegacyV: float64(legacyVoltage) / 10,
		DrawnMAh:       drawn,
		RSSI:           rssi,
		AmperageA:      float64(amperage) / 100,
		VoltageV:       float64(voltage) / 100,
	}, nil
}
