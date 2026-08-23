package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestDecodeGPSStatusParts(t *testing.T) {
	config, err := DecodeGPSConfig([]byte{1, 0, 1, 1, 1, 1})
	if err != nil {
		t.Fatalf("DecodeGPSConfig() error = %v", err)
	}
	if config.Provider != 1 || !config.AutoConfig || !config.AutoBaud {
		t.Fatalf("config = %+v", config)
	}
	if config.HomePointOnce == nil || !*config.HomePointOnce || config.UBloxUseGalileo == nil || !*config.UBloxUseGalileo {
		t.Fatalf("config = %+v", config)
	}

	positionPayload := []byte{1, 12}
	positionPayload = appendS32Test(positionPayload, 599123456)
	positionPayload = appendS32Test(positionPayload, 105123456)
	positionPayload = appendU16Test(positionPayload, 124)
	positionPayload = appendU16Test(positionPayload, 1450)
	positionPayload = appendU16Test(positionPayload, 2715)
	positionPayload = appendU16Test(positionPayload, 95)
	position, err := DecodeGPSPosition(positionPayload)
	if err != nil {
		t.Fatalf("DecodeGPSPosition() error = %v", err)
	}
	if !position.Fix || position.Satellites != 12 || position.LatitudeDegrees != 59.9123456 || position.LongitudeDegrees != 10.5123456 {
		t.Fatalf("position = %+v", position)
	}
	if position.GroundCourseDeg != 271.5 || position.PDOP == nil || *position.PDOP != 95 {
		t.Fatalf("position = %+v", position)
	}

	home, err := DecodeGPSHome([]byte{0x56, 0x01, 0xb8, 0x00, 1})
	if err != nil {
		t.Fatalf("DecodeGPSHome() error = %v", err)
	}
	if home.DistanceM != 342 || home.DirectionDeg != 184 || !home.Update {
		t.Fatalf("home = %+v", home)
	}
}

func TestEncodeGPSConfig(t *testing.T) {
	homePointOnce := true
	useGalileo := false
	payload := EncodeGPSConfig(GPSConfig{
		Provider:        1,
		SBASMode:        2,
		AutoConfig:      true,
		AutoBaud:        false,
		HomePointOnce:   &homePointOnce,
		UBloxUseGalileo: &useGalileo,
	})
	want := []byte{1, 2, 1, 0, 1, 0}
	if !bytes.Equal(payload, want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestDecodeGPSRescue(t *testing.T) {
	payload := appendU16Test(nil, 3200)
	payload = appendU16Test(payload, 100)
	payload = appendU16Test(payload, 50)
	payload = appendU16Test(payload, 1500)
	payload = appendU16Test(payload, 1200)
	payload = appendU16Test(payload, 1800)
	payload = appendU16Test(payload, 1450)
	payload = append(payload, 1, 8)
	payload = appendU16Test(payload, 500)
	payload = appendU16Test(payload, 150)
	payload = append(payload, 1, 2)
	payload = appendU16Test(payload, 30)
	payload = appendU16Test(payload, 20)
	rescue, err := DecodeGPSRescue(payload)
	if err != nil {
		t.Fatalf("DecodeGPSRescue() error = %v", err)
	}
	if rescue.MaxRescueAngle != 3200 || rescue.MinSats != 8 || rescue.ThrottleHover != 1450 {
		t.Fatalf("rescue = %+v", rescue)
	}
	if rescue.AscendRate == nil || *rescue.AscendRate != 500 || rescue.DescendRate == nil || *rescue.DescendRate != 150 {
		t.Fatalf("rescue = %+v", rescue)
	}
	if rescue.AllowArmingWithoutFix == nil || !*rescue.AllowArmingWithoutFix || rescue.InitialClimbM == nil || *rescue.InitialClimbM != 20 {
		t.Fatalf("rescue = %+v", rescue)
	}
	payload = EncodeGPSRescue(*rescue)
	if len(payload) != 26 || payload[0] != 0x80 || payload[1] != 0x0c || payload[24] != 20 || payload[25] != 0 {
		t.Fatalf("payload = %v", payload)
	}
}

func TestDecodeGPSRescuePIDAndSatellites(t *testing.T) {
	pidPayload := appendU16Test(nil, 80)
	pidPayload = appendU16Test(pidPayload, 10)
	pidPayload = appendU16Test(pidPayload, 5)
	pidPayload = appendU16Test(pidPayload, 120)
	pidPayload = appendU16Test(pidPayload, 20)
	pidPayload = appendU16Test(pidPayload, 10)
	pidPayload = appendU16Test(pidPayload, 45)
	pid, err := DecodeGPSRescuePID(pidPayload)
	if err != nil {
		t.Fatalf("DecodeGPSRescuePID() error = %v", err)
	}
	if pid.AltitudeP != 80 || pid.VelocityP != 120 || pid.YawP != 45 {
		t.Fatalf("pid = %+v", pid)
	}
	encodedPID := EncodeGPSRescuePID(*pid)
	if len(encodedPID) != 14 || encodedPID[0] != 80 || encodedPID[6] != 120 || encodedPID[12] != 45 {
		t.Fatalf("encoded pid = %v", encodedPID)
	}

	satellites, err := DecodeGPSSatellites([]byte{2, 0, 12, 4, 45, 1, 24, 3, 39})
	if err != nil {
		t.Fatalf("DecodeGPSSatellites() error = %v", err)
	}
	if len(satellites) != 2 || satellites[1].SVID != 24 || satellites[1].CNO != 39 {
		t.Fatalf("satellites = %+v", satellites)
	}
}

func appendS32Test(dst []byte, v int32) []byte {
	return appendU32Test(dst, uint32(v))
}

func TestDecodeGPSStatistics(t *testing.T) {
	stats, err := DecodeGPSStatistics([]byte{0x01, 0x02, 0xff})
	if err != nil {
		t.Fatalf("DecodeGPSStatistics() error = %v", err)
	}
	if stats.PayloadLen != 3 || stats.PayloadHex != "0102ff" {
		t.Fatalf("statistics = %+v", stats)
	}
}

func TestDecodeGPSStatisticsRejectsEmptyPayload(t *testing.T) {
	if _, err := DecodeGPSStatistics(nil); err == nil {
		t.Fatal("DecodeGPSStatistics() error = nil, want empty payload error")
	}
}

func TestReadGPSStatusWithFakeFC(t *testing.T) {
	client, err := connection.NewClient(fakefc.New(), time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	status, warnings, err := ReadGPSStatus(context.Background(), client)
	if err != nil {
		t.Fatalf("ReadGPSStatus() error = %v", err)
	}
	if len(status.Satellites) != 2 {
		t.Fatalf("len(status.Satellites) = %d, want 2 (status = %+v)", len(status.Satellites), status)
	}
	var found bool
	for _, warning := range warnings {
		if strings.Contains(warning, "gps statistics unavailable") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings %v do not report gps statistics unavailability", warnings)
	}
}
