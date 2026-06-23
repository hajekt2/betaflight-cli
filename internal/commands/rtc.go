package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type RTCStatus struct {
	Available bool   `json:"available"`
	Year      uint16 `json:"year,omitempty"`
	Month     uint8  `json:"month,omitempty"`
	Day       uint8  `json:"day,omitempty"`
	Hour      uint8  `json:"hour,omitempty"`
	Minute    uint8  `json:"minute,omitempty"`
	Second    uint8  `json:"second,omitempty"`
	Millis    uint16 `json:"millis,omitempty"`
	ISOUTC    string `json:"iso_utc,omitempty"`
	Source    string `json:"source,omitempty"`
}

type RTCSetResult struct {
	TimestampUTC string `json:"timestamp_utc"`
	MSPCode      uint16 `json:"msp_code"`
	MSPName      string `json:"msp_name"`
	Acknowledged bool   `json:"acknowledged"`
}

func ReadRTCStatus(ctx context.Context, client *connection.Client) (*RTCStatus, []string, error) {
	frame, err := client.Request(ctx, msp.MSPRtc, nil)
	if err != nil {
		return nil, []string{fmt.Sprintf("MSP_RTC unavailable: %v", err)}, fmt.Errorf("rtc status unavailable")
	}
	rtc, err := DecodeRTC(frame.Payload)
	if err != nil {
		return nil, []string{fmt.Sprintf("MSP_RTC decode failed: %v", err)}, fmt.Errorf("rtc status unavailable")
	}
	rtc.Source = "MSP_RTC"
	warnings := []string{}
	if !rtc.Available {
		warnings = append(warnings, "MSP_RTC returned no datetime; RTC is unavailable or not set")
	}
	return rtc, warnings, nil
}

func SetRTC(ctx context.Context, client *connection.Client, timestamp time.Time) (*RTCSetResult, error) {
	utc := timestamp.UTC()
	payload, err := EncodeRTC(utc)
	if err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetRtc, payload); err != nil {
		return nil, fmt.Errorf("rtc set request failed: %w", err)
	}
	return &RTCSetResult{
		TimestampUTC: utc.Format("2006-01-02T15:04:05.000Z"),
		MSPCode:      msp.MSPSetRtc,
		MSPName:      "MSP_SET_RTC",
		Acknowledged: true,
	}, nil
}

func DecodeRTC(payload []byte) (*RTCStatus, error) {
	if len(payload) == 0 {
		return &RTCStatus{Available: false}, nil
	}
	const length = 9
	if len(payload) < length {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_RTC size %d", len(payload), length)
	}
	r := msp.NewPayloadReader(payload)
	year, err := r.U16()
	if err != nil {
		return nil, msp.RequireNoShort(err, "rtc year")
	}
	month, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "rtc month")
	}
	day, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "rtc day")
	}
	hour, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "rtc hour")
	}
	minute, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "rtc minute")
	}
	second, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "rtc second")
	}
	millis, err := r.U16()
	if err != nil {
		return nil, msp.RequireNoShort(err, "rtc millis")
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_RTC returned %d trailing byte(s)", r.Remaining())
	}
	t := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), int(millis)*int(time.Millisecond), time.UTC)
	if t.Year() != int(year) || t.Month() != time.Month(month) || t.Day() != int(day) || t.Hour() != int(hour) || t.Minute() != int(minute) || t.Second() != int(second) {
		return nil, fmt.Errorf("MSP_RTC returned invalid datetime")
	}
	return &RTCStatus{
		Available: true,
		Year:      year,
		Month:     month,
		Day:       day,
		Hour:      hour,
		Minute:    minute,
		Second:    second,
		Millis:    millis,
		ISOUTC:    t.Format("2006-01-02T15:04:05.000Z"),
	}, nil
}

func EncodeRTC(timestamp time.Time) ([]byte, error) {
	utc := timestamp.UTC()
	year := utc.Year()
	if year < 0 || year > 65535 {
		return nil, fmt.Errorf("rtc year %d is outside uint16 range", year)
	}
	millis := utc.Nanosecond() / int(time.Millisecond)
	payload := make([]byte, 0, 9)
	payload = append(payload, byte(year), byte(year>>8))
	payload = append(payload, byte(utc.Month()), byte(utc.Day()), byte(utc.Hour()), byte(utc.Minute()), byte(utc.Second()))
	payload = append(payload, byte(millis), byte(millis>>8))
	return payload, nil
}
