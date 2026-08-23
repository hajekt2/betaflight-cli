package blackbox

import (
	"math"
	"strings"
	"testing"
)

func TestFFTRadix2SinePeakBin(t *testing.T) {
	const n = 64
	const peakBin = 8
	re := make([]float64, n)
	im := make([]float64, n)
	for i := range n {
		re[i] = math.Sin(2 * math.Pi * float64(peakBin) * float64(i) / float64(n))
	}
	fftRadix2(re, im)
	bestBin, bestMag := -1, 0.0
	for k := 1; k < n/2; k++ {
		mag := math.Hypot(re[k], im[k])
		if mag > bestMag {
			bestMag = mag
			bestBin = k
		}
	}
	if bestBin != peakBin {
		t.Fatalf("FFT peak bin = %d, want %d", bestBin, peakBin)
	}
	// A unit sine spreads half its amplitude to the conjugate bin.
	if math.Abs(bestMag-float64(n)/2) > 1e-6 {
		t.Fatalf("peak magnitude = %f, want %f", bestMag, float64(n)/2)
	}
}

func TestWelchPSDBandSplitCompositeSine(t *testing.T) {
	const fs = 2048.0
	const segment = 256 // bin width 8 Hz
	n := 4096
	x := make([]float64, n)
	for i := range n {
		t := float64(i) / fs
		x[i] = 2*math.Sin(2*math.Pi*48*t) + math.Sin(2*math.Pi*400*t) + math.Sin(2*math.Pi*896*t)
	}
	psd, segments := welchPSD(x, fs, segment)
	if segments == 0 {
		t.Fatal("welchPSD produced no segments")
	}
	bands := spectralBands(psd, fs, segment)
	if bands.Below100Hz <= 5*bands.Hz100To300 {
		t.Fatalf("below-100Hz band should dominate 100-300Hz: %+v", bands)
	}
	if bands.Hz300To600 <= 5*bands.Hz100To300 {
		t.Fatalf("300-600Hz band should dominate 100-300Hz: %+v", bands)
	}
	if bands.Above600Hz <= 5*bands.Hz100To300 {
		t.Fatalf("above-600Hz band should dominate 100-300Hz: %+v", bands)
	}
	if bands.Hz300To600 <= 5*bands.Hz100To300 {
		t.Fatalf("300-600Hz band should dominate 100-300Hz leakage: %+v", bands)
	}
	peakHz, _ := dominantSpectralPeak(psd, fs, segment)
	// The doubled-amplitude 48 Hz tone must dominate; leakage must not.
	if peakHz < 40 || peakHz > 56 {
		t.Fatalf("dominant peak = %f Hz, want near 48 Hz", peakHz)
	}
}

func TestStepResponseOvershootOnIdealSecondOrderArray(t *testing.T) {
	pre := make([]float64, 16) // baseline zeros before the step
	post := []float64{
		0.25, 0.55, 0.85, 1.08, 1.24, 1.30, 1.18, 1.12,
		1.03, 1.01, 0.99, 1.0, 1.0, 1.0,
		1.0, 1.0, 1.0, 1.0, 1.0, 1.0,
	}
	response := append(pre, post...)
	edge := stepEdge{sampleIndex: len(pre), delta: 1.0}

	overshoot, settleSamples, ok := analyzeSingleStep(response, edge)
	if !ok {
		t.Fatal("analyzeSingleStep did not settle")
	}
	// Peak 1.30 settles to ~1.0 with a unit step: ~30% overshoot.
	if overshoot < 29 || overshoot > 31 {
		t.Fatalf("overshoot = %f%%, want ~30%%", overshoot)
	}
	if settleSamples != 8 {
		t.Fatalf("settle samples = %f, want 8 (first stable entry into +/-10%% band)", settleSamples)
	}
}

func TestFindStepEdgesDetectsTransitions(t *testing.T) {
	setpoint := make([]float64, 600)
	fillRange(setpoint, 80, 240, 500)
	fillRange(setpoint, 240, 400, 100)
	fillRange(setpoint, 400, 600, 700)
	// Sub-threshold wiggle must not register.
	setpoint[500] += 50

	edges := findStepEdges(setpoint, 0)
	if len(edges) != 3 {
		t.Fatalf("edges = %+v, want 3", edges)
	}
	wantIndexes := []int{80, 240, 400}
	for i, want := range wantIndexes {
		if edges[i].sampleIndex != want {
			t.Fatalf("edge %d at %d, want %d", i, edges[i].sampleIndex, want)
		}
	}
}

func TestMotorBalanceFlatArrays(t *testing.T) {
	values := repeatFloat(1500, 200)
	motors := []axisSeries{
		{index: 0, field: "motor[0]", values: values},
		{index: 1, field: "motor[1]", values: repeatFloat(1500, 200)},
	}
	analysis := analyzeMotorBalance(motors, AnalysisOptions{})
	if analysis == nil {
		t.Fatal("analyzeMotorBalance returned nil for flat arrays")
	}
	if len(analysis.Motors) != 2 {
		t.Fatalf("motors = %+v", analysis.Motors)
	}
	for _, stats := range analysis.Motors {
		if math.Abs(stats.Mean-1500) > 1e-9 || stats.StdDev != 0 {
			t.Fatalf("stats = %+v, want mean 1500 stddev 0", stats)
		}
	}
	if analysis.MaxSpreadPercent == nil || *analysis.MaxSpreadPercent > 0.001 {
		t.Fatalf("max spread = %+v, want ~0", analysis.MaxSpreadPercent)
	}
	if analysis.OutlierMotorIndex != nil {
		t.Fatalf("outlier = %v, want none on flat arrays", *analysis.OutlierMotorIndex)
	}
}

func TestMotorBalanceFlagsOutlier(t *testing.T) {
	good := repeatFloat(1500, 200)
	bad := make([]float64, 200)
	for i := range bad {
		bad[i] = 1650 + float64(i%3)
	}
	motors := []axisSeries{
		{index: 0, field: "motor[0]", values: append([]float64(nil), good...)},
		{index: 1, field: "motor[1]", values: append([]float64(nil), good...)},
		{index: 2, field: "motor[2]", values: bad},
	}
	analysis := analyzeMotorBalance(motors, AnalysisOptions{})
	if analysis == nil {
		t.Fatal("analyzeMotorBalance returned nil")
	}
	if analysis.OutlierMotorIndex == nil {
		t.Fatal("outlier missing, want motor 2")
	}
	if *analysis.OutlierMotorIndex != 2 {
		t.Fatalf("outlier = %d, want motor 2", *analysis.OutlierMotorIndex)
	}
	if analysis.MaxSpreadPercent == nil || *analysis.MaxSpreadPercent < 5 {
		t.Fatalf("max spread = %+v, want > 5%%", analysis.MaxSpreadPercent)
	}
}

// --- end-to-end Analyze on a synthesized minimal .BFL ---------------------------

func encVB(value uint32) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if value == 0 {
			return out
		}
	}
}

func encSignedVB(value int) []byte {
	var encoded uint32
	if value >= 0 {
		encoded = uint32(value) << 1
	} else {
		encoded = uint32(-value)<<1 - 1
	}
	return encVB(encoded)
}

type sampleRow struct {
	time     int
	gyro0    int
	gyro1    int
	setpoint int
	motor0   int
	motor1   int
}

func buildAnalyzeLog(rows []sampleRow) string {
	header := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Craft name:Analyzer Test",
		"H looptime:1000",
		"H Field I name:time,gyroADC[0],gyroADC[1],setpoint[0],motor[0],motor[1]",
		"H Field I signed:0,1,1,1,0,0",
		"H Field I predictor:0,0,0,0,0,0",
		"H Field I encoding:1,1,1,1,1,1",
		"H Field P predictor:0,0,0,0,0,0",
		"H Field P encoding:1,1,1,1,1,1",
	}, "\n")

	var body []byte
	appendRow := func(marker byte, row sampleRow) {
		payload := []byte{}
		payload = append(payload, encVB(uint32(row.time))...)
		payload = append(payload, encSignedVB(row.gyro0)...)
		payload = append(payload, encSignedVB(row.gyro1)...)
		payload = append(payload, encSignedVB(row.setpoint)...)
		payload = append(payload, encVB(uint32(row.motor0))...)
		payload = append(payload, encVB(uint32(row.motor1))...)
		body = append(body, marker)
		body = append(body, payload...)
	}
	for i, row := range rows {
		if i == 0 {
			appendRow('I', row)
			continue
		}
		appendRow('P', row)
	}
	return header + "\n" + string(body)
}

func synthAnalyzeRows(n int) []sampleRow {
	rows := make([]sampleRow, n)
	for k := range n {
		setpoint := 0
		switch {
		case k >= 400:
			setpoint = 700
		case k >= 240:
			setpoint = 100
		case k >= 80:
			setpoint = 500
		}
		rows[k] = sampleRow{
			time:     k * 1000,
			gyro0:    setpoint / 2,
			gyro1:    int(math.Round(150 * math.Sin(2*math.Pi*125*float64(k)/1000))),
			setpoint: setpoint,
			motor0:   1500 + setpoint/20,
			motor1:   1505 + setpoint/20,
		}
	}
	return rows
}

func TestAnalyzeMinimalSynthesizedLog(t *testing.T) {
	rows := synthAnalyzeRows(512)
	log := buildAnalyzeLog(rows)
	analysis, err := Analyze(strings.NewReader(log), AnalysisOptions{})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if analysis.Meta.SampleCount != 512 {
		t.Fatalf("sample count = %d, want 512", analysis.Meta.SampleCount)
	}
	if math.Abs(analysis.Meta.AxisRateHz-1000) > 0.01 {
		t.Fatalf("axis rate = %f, want 1000", analysis.Meta.AxisRateHz)
	}
	if math.Abs(analysis.Meta.DurationS-0.511) > 0.001 {
		t.Fatalf("duration = %f, want 0.511", analysis.Meta.DurationS)
	}

	if analysis.GyroNoise == nil {
		t.Fatal("gyro noise analysis missing")
	}
	axisSine, ok := analysis.GyroNoise.PerAxis["gyroADC[1]"]
	if !ok {
		t.Fatalf("per-axis keys = %+v", analysis.GyroNoise.PerAxis)
	}
	// The isolated 125 Hz tone on gyroADC[1] lands exactly on Welch bin 32
	// (1000 Hz rate, 256-sample segments, ~3.9 Hz bins).
	if axisSine.DominantPeakHz != 125 {
		t.Fatalf("dominant peak = %f Hz, want 125", axisSine.DominantPeakHz)
	}
	b := axisSine.Bands
	if b.Hz100To300 <= 10*b.Hz300To600 || b.Hz100To300 <= 10*b.Above600Hz || b.Hz100To300 <= 10*b.Below100Hz {
		t.Fatalf("125Hz component should dominate all other bands: %+v", b)
	}

	if analysis.StepResponse == nil {
		t.Fatal("step response analysis missing")
	}
	if analysis.StepResponse.EdgesAnalyzed != 3 {
		t.Fatalf("edges analyzed = %d, want 3", analysis.StepResponse.EdgesAnalyzed)
	}
	if analysis.StepResponse.MedianOvershootPercent == nil || analysis.StepResponse.MedianSettleMs == nil {
		t.Fatal("step response medians missing")
	}
	spAxis, ok := analysis.StepResponse.PerAxis["gyroADC[0]"]
	if !ok || spAxis.OvershootPercent == nil || spAxis.SettleMs == nil {
		t.Fatalf("step response per-axis = %+v", analysis.StepResponse.PerAxis)
	}

	if analysis.MotorBalance == nil {
		t.Fatal("motor balance analysis missing")
	}
	if len(analysis.MotorBalance.Motors) != 2 {
		t.Fatalf("motor balance motors = %+v", analysis.MotorBalance.Motors)
	}
	if analysis.MotorBalance.OutlierMotorIndex != nil {
		t.Fatalf("unexpected outlier motor %d", *analysis.MotorBalance.OutlierMotorIndex)
	}
	if len(analysis.Caveats) != 0 {
		t.Fatalf("unexpected caveats = %+v", analysis.Caveats)
	}
}

func TestAnalyzeCaveatsWhenStreamsAbsent(t *testing.T) {
	rows := make([]sampleRow, 64)
	for k := range rows {
		rows[k] = sampleRow{time: k * 1000, motor0: 1400, motor1: 1410}
	}
	log := buildMinimalMotorOnlyLog(rows)
	analysis, err := Analyze(strings.NewReader(log), AnalysisOptions{})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if analysis.Meta.SampleCount != 64 {
		t.Fatalf("sample count = %d, want 64", analysis.Meta.SampleCount)
	}
	if analysis.GyroNoise != nil {
		t.Fatal("gyro noise should degrade to nil without gyro streams")
	}
	if analysis.StepResponse != nil {
		t.Fatal("step response should degrade to nil without setpoint streams")
	}
	if analysis.MotorBalance == nil {
		t.Fatal("motor balance should be computed for two flat motors")
	}
	if len(analysis.Caveats) < 2 {
		t.Fatalf("caveats = %+v, want entries for gyro and step metrics", analysis.Caveats)
	}
}

func buildMinimalMotorOnlyLog(rows []sampleRow) string {
	header := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H looptime:1000",
		"H Field I name:time,motor[0],motor[1]",
		"H Field I signed:0,0,0",
		"H Field I predictor:0,0,0",
		"H Field I encoding:1,1,1",
		"H Field P predictor:0,0,0",
		"H Field P encoding:1,1,1",
	}, "\n")
	var body []byte
	appendRow := func(marker byte, row sampleRow) {
		payload := encVB(uint32(row.time))
		payload = append(payload, encVB(uint32(row.motor0))...)
		payload = append(payload, encVB(uint32(row.motor1))...)
		body = append(append(body, marker), payload...)
	}
	for i, row := range rows {
		marker := byte('P')
		if i == 0 {
			marker = 'I'
		}
		appendRow(marker, row)
	}
	return header + "\n" + string(body)
}

func fillRange(values []float64, start, end int, fill float64) {
	for i := start; i < end; i++ {
		values[i] = fill
	}
}

func repeatFloat(value float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = value
	}
	return out
}
