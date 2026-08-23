package blackbox

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// AnalysisOptions tunes Analyze. Zero values select defaults.
type AnalysisOptions struct {
	// MaxSamples caps the number of decoded main frames (0 = unlimited).
	MaxSamples int
	// SegmentSize is the Welch PSD segment length in samples; must be a power of two (default 256).
	SegmentSize int
	// StepDeltaFraction is the relative setpoint jump treated as a step edge (default 0.2).
	StepDeltaFraction float64
	// MinStepEdges is the minimum number of edges required before step metrics are reported (default 3).
	MinStepEdges int
	// MotorSpreadThresholdPercent is the per-motor spread above which an outlier is flagged (default 5).
	MotorSpreadThresholdPercent float64
}

const (
	defaultWelchSegment     = 256
	defaultMinStepEdges     = 3
	defaultStepDeltaFrac    = 0.2
	defaultMotorSpreadLimit = 5.0
	minWelchSegment         = 16
	stepResponseWindow      = 128
)

type Analysis struct {
	Meta         AnalysisMeta          `json:"meta"`
	GyroNoise    *GyroNoiseAnalysis    `json:"gyro_noise,omitempty"`
	StepResponse *StepResponseAnalysis `json:"step_response,omitempty"`
	MotorBalance *MotorBalanceAnalysis `json:"motor_balance,omitempty"`
	Caveats      []string              `json:"caveats,omitempty"`
	Warnings     []string              `json:"warnings,omitempty"`
}

type AnalysisMeta struct {
	SampleCount int     `json:"sample_count"`
	DurationS   float64 `json:"duration_s"`
	AxisRateHz  float64 `json:"axis_rate_hz"`
}

type SpectralBands struct {
	Below100Hz float64 `json:"lt_100hz"`
	Hz100To300 float64 `json:"hz_100_to_300"`
	Hz300To600 float64 `json:"hz_300_to_600"`
	Above600Hz float64 `json:"gt_600hz"`
}

type GyroNoiseAxis struct {
	Samples        int           `json:"samples"`
	DominantPeakHz float64       `json:"dominant_peak_hz"`
	PeakPower      float64       `json:"peak_power"`
	Bands          SpectralBands `json:"bands"`
}

type GyroNoiseAnalysis struct {
	PerAxis map[string]GyroNoiseAxis `json:"per_axis"`
}

type StepResponseAxis struct {
	OvershootPercent *float64 `json:"overshoot_percent,omitempty"`
	SettleMs         *float64 `json:"settle_ms,omitempty"`
}

type StepResponseAnalysis struct {
	PerAxis                map[string]StepResponseAxis `json:"per_axis"`
	EdgesAnalyzed          int                         `json:"edges_analyzed"`
	MedianOvershootPercent *float64                    `json:"median_overshoot_percent,omitempty"`
	MedianSettleMs         *float64                    `json:"median_settle_ms,omitempty"`
}

type MotorStats struct {
	Index   int     `json:"index"`
	Mean    float64 `json:"mean"`
	StdDev  float64 `json:"stddev"`
	Samples int     `json:"samples"`
}

type MotorBalanceAnalysis struct {
	Motors            []MotorStats `json:"motors"`
	MaxSpreadPercent  *float64     `json:"max_spread_percent,omitempty"`
	OutlierMotorIndex *int         `json:"outlier_motor_index,omitempty"`
}

// Analyze decodes a Blackbox log and computes offline flight-performance metrics.
// Metrics degrade to nil with a caveat when their required streams are absent.
func Analyze(r io.Reader, opts AnalysisOptions) (Analysis, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Analysis{}, err
	}
	if len(data) == 0 {
		return Analysis{}, fmt.Errorf("blackbox log is empty")
	}
	inspection := Inspection{
		Headers:                 map[string]string{},
		FieldDefinitions:        map[string]FieldDefinition{},
		FrameMarkerCountsApprox: map[string]int{},
	}
	headerEnd, err := parseHeaders(data, &inspection)
	if err != nil {
		return Analysis{}, err
	}
	body := data[headerEnd:]

	series, sampleCount, decodeWarnings := decodeAllMainFrames(body, inspection.Headers, inspection.FieldDefinitions, opts.MaxSamples)

	out := Analysis{
		Meta:     buildMeta(series, sampleCount, inspection.Headers),
		Warnings: decodeWarnings,
	}
	if sampleCount == 0 {
		out.Caveats = append(out.Caveats, "no decodable main frames found; no metrics computed")
		return out, nil
	}

	gyroAxes, setpointAxes, motorAxes := splitAxes(series)

	gyroNoise, gyroCaveat, err := analyzeGyroNoise(gyroAxes, out.Meta.AxisRateHz, opts)
	if err != nil {
		return Analysis{}, err
	}
	out.GyroNoise = gyroNoise
	if gyroCaveat != "" {
		out.Caveats = append(out.Caveats, gyroCaveat)
	} else if out.GyroNoise == nil {
		out.Caveats = append(out.Caveats, "gyro streams absent or too short for spectral analysis; gyro noise skipped")
	}
	step, stepCaveat := analyzeStepResponse(setpointAxes, gyroAxes, out.Meta.AxisRateHz, opts)
	out.StepResponse = step
	if out.StepResponse == nil {
		out.Caveats = append(out.Caveats, fmt.Sprintf("setpoint/gyro streams absent or fewer than %d detectable step edges; step response skipped", minStepEdgeCount(opts)))
	} else if stepCaveat != "" {
		out.Caveats = append(out.Caveats, stepCaveat)
	}
	out.MotorBalance = analyzeMotorBalance(motorAxes, opts)
	if out.MotorBalance == nil {
		out.Caveats = append(out.Caveats, "motor streams absent; motor balance skipped")
	}
	sort.Strings(out.Caveats)
	return out, nil
}

// decodeAllMainFrames walks every frame in the log body and collects per-field
// value series from I/P frames, stopping after maxSamples frames when positive.
func decodeAllMainFrames(data []byte, headers map[string]string, definitions map[string]FieldDefinition, maxSamples int) (map[string][]float64, int, []string) {
	series := map[string][]float64{}
	ctx := newDecodeContext(headers, definitions)
	pos := 0
	sampleCount := 0
	var warnings []string
	failed := 0
	for pos < len(data) {
		candidate, next, nextCtx, ok := findNextDecodedCandidate(data, pos, 0, definitions, ctx, 256)
		if !ok {
			pos++
			continue
		}
		pos = next
		ctx = nextCtx
		if candidate.Type != "I" && candidate.Type != "P" {
			continue
		}
		if maxSamples > 0 && sampleCount >= maxSamples {
			warnings = append(warnings, fmt.Sprintf("analysis covers only the first %d main frames (MaxSamples cap); remaining frames ignored", maxSamples))
			break
		}
		payload, ok := candidatePayload(data, candidate)
		if !ok {
			continue
		}
		def, ok := definitionForFrame(candidate.Type, definitions)
		if !ok {
			continue
		}
		frame, err := decodeFramePayload(&ctx, candidate.Type, candidate, payload, def)
		if err != nil {
			failed++
			continue
		}
		sampleCount++
		fields := make([]string, 0, len(frame.Values))
		for field := range frame.Values {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			series[field] = append(series[field], float64(frame.Values[field]))
		}
	}
	if failed > 0 {
		warnings = append(warnings, fmt.Sprintf("%d main frame(s) failed to decode and were excluded", failed))
	}
	return series, sampleCount, warnings
}

func buildMeta(series map[string][]float64, sampleCount int, headers map[string]string) AnalysisMeta {
	meta := AnalysisMeta{SampleCount: sampleCount}
	timeSeries := firstPresent(series, "time", "Time")
	rate := axisFrameRate(timeSeries, headers)
	if len(timeSeries) >= 2 {
		durationUs := timeSeries[len(timeSeries)-1] - timeSeries[0]
		meta.DurationS = durationUs / 1e6
	} else if rate > 0 {
		meta.DurationS = float64(sampleCount) / rate
	}
	meta.AxisRateHz = rate
	return meta
}

// axisFrameRate derives the per-axis sampling rate in Hz. A decoded time
// stream with at least two monotonic samples wins: the median of consecutive
// deltas reflects the actual logged frame cadence even when it is decimated
// relative to the loop rate. Otherwise fall back to the looptime header,
// scaled by the blackbox P-interval logging decimation when present.
func axisFrameRate(timeSeries []float64, headers map[string]string) float64 {
	if len(timeSeries) >= 2 {
		deltas := make([]float64, 0, len(timeSeries)-1)
		monotonic := true
		for i := 1; i < len(timeSeries); i++ {
			delta := timeSeries[i] - timeSeries[i-1]
			if delta <= 0 {
				monotonic = false
				break
			}
			deltas = append(deltas, delta)
		}
		if monotonic && len(deltas) > 0 {
			return 1e6 / medianFloat(deltas)
		}
	}
	rate := loopFrequency(headers)
	if rate <= 0 {
		return 0
	}
	if factor := pIntervalFactor(headers); factor > 0 {
		rate *= factor
	}
	return rate
}

func loopFrequency(headers map[string]string) float64 {
	raw := strings.TrimSpace(headers["looptime"])
	if raw == "" {
		return 0
	}
	loopUs, err := strconv.ParseFloat(raw, 64)
	if err != nil || loopUs <= 0 {
		return 0
	}
	return 1e6 / loopUs
}

// pIntervalFactor parses the blackbox logging decimation header into a
// multiplier applied to the loop rate. Betaflight writes 'P interval:num/denom'
// meaning every denom-th loop iteration (times num) produces one logged frame;
// a bare integer N means every Nth frame. Returns 0 when no usable header exists.
func pIntervalFactor(headers map[string]string) float64 {
	if raw := strings.TrimSpace(headers["P interval"]); raw != "" {
		if numStr, denomStr, ok := strings.Cut(raw, "/"); ok {
			num, err1 := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
			denom, err2 := strconv.ParseFloat(strings.TrimSpace(denomStr), 64)
			if err1 == nil && err2 == nil && num > 0 && denom > 0 {
				return num / denom
			}
			return 0
		}
		if n, err := strconv.ParseFloat(raw, 64); err == nil && n > 0 {
			return 1 / n
		}
		return 0
	}
	var num, denom = 1.0, 1.0
	sawAny := false
	if raw := strings.TrimSpace(headers["P num"]); raw != "" {
		sawAny = true
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 {
			return 0
		}
		num = v
	}
	if raw := strings.TrimSpace(headers["P denom"]); raw != "" {
		sawAny = true
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 {
			return 0
		}
		denom = v
	}
	if !sawAny {
		return 0
	}
	return num / denom
}

func firstPresent(series map[string][]float64, names ...string) []float64 {
	for _, name := range names {
		if s, ok := series[name]; ok && len(s) > 0 {
			return s
		}
	}
	return nil
}

// axisKey splits a grouped field name into its base name and trailing [n] index.
func axisKey(field string) (string, int, bool) {
	open := strings.LastIndex(field, "[")
	if open < 0 || !strings.HasSuffix(field, "]") {
		return strings.ToLower(field), 0, false
	}
	idx, err := strconv.Atoi(field[open+1 : len(field)-1])
	if err != nil {
		return strings.ToLower(field), 0, false
	}
	return strings.ToLower(field[:open]), idx, true
}

type axisSeries struct {
	index  int
	field  string
	values []float64
}

// splitAxes partitions collected series into gyro/setpoint/motor axes using the
// same field classification conventions as classifyStreamField.
func splitAxes(series map[string][]float64) (map[int]axisSeries, map[int]axisSeries, []axisSeries) {
	gyros := map[int]axisSeries{}
	setpoints := map[int]axisSeries{}
	motors := []axisSeries{}
	names := make([]string, 0, len(series))
	for name := range series {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		base, idx, indexed := axisKey(name)
		switch {
		case strings.HasPrefix(base, "gyro"):
			key := idx
			if !indexed {
				continue
			}
			gyros[key] = axisSeries{index: key, field: name, values: series[name]}
		case strings.HasPrefix(base, "setpoint"):
			if !indexed {
				continue
			}
			setpoints[idx] = axisSeries{index: idx, field: name, values: series[name]}
		case base == "motor" || base == "motoroutput":
			motors = append(motors, axisSeries{index: idx, field: name, values: series[name]})
		}
	}
	return gyros, setpoints, motors
}

// --- Welch PSD + radix-2 FFT -------------------------------------------------

// fftRadix2 computes an in-place iterative Cooley-Tukey FFT.
// Length must be a power of two.
func fftRadix2(re, im []float64) {
	n := len(re)
	if n <= 1 {
		return
	}
	// Bit-reversal permutation.
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		wRe := math.Cos(angle)
		wIm := math.Sin(angle)
		for start := 0; start < n; start += length {
			curRe, curIm := 1.0, 0.0
			for k := 0; k < length/2; k++ {
				evenIdx := start + k
				oddIdx := start + k + length/2
				oddRe := re[oddIdx]*curRe - im[oddIdx]*curIm
				oddIm := re[oddIdx]*curIm + im[oddIdx]*curRe
				re[oddIdx] = re[evenIdx] - oddRe
				im[oddIdx] = im[evenIdx] - oddIm
				re[evenIdx] += oddRe
				im[evenIdx] += oddIm
				nextRe := curRe*wRe - curIm*wIm
				curIm = curRe*wIm + curIm*wRe
				curRe = nextRe
			}
		}
	}
}

// welchPSD estimates the power spectral density of x sampled at fs Hz using a
// Hann-windowed Welch average with 50% segment overlap. It returns the averaged
// periodogram (one value per bin, bin width fs/effectiveSegment) and the
// effective FFT size actually used, which falls back to prevPowerOfTwo(len(x))
// when the requested segment is too small or longer than x.
func welchPSD(x []float64, fs float64, segmentSize int) ([]float64, int) {
	n := segmentSize
	if n < minWelchSegment || len(x) < n {
		n = prevPowerOfTwo(len(x))
	}
	if n < minWelchSegment || fs <= 0 {
		return nil, 0
	}
	window := hannWindow(n)
	// Normalizer so that a unit-amplitude sine at bin center yields ~power A^2/2.
	windowPower := 0.0
	for _, w := range window {
		windowPower += w * w
	}
	normalizer := fs * windowPower

	hop := n / 2
	psd := make([]float64, n)
	segments := 0
	re := make([]float64, n)
	im := make([]float64, n)
	for start := 0; start+n <= len(x); start += hop {
		mean := 0.0
		for _, v := range x[start : start+n] {
			mean += v
		}
		mean /= float64(n)
		for i, v := range x[start : start+n] {
			re[i] = (v - mean) * window[i]
			im[i] = 0
		}
		fftRadix2(re, im)
		for k := range psd {
			psd[k] += (re[k]*re[k] + im[k]*im[k]) / normalizer
		}
		segments++
	}
	if segments == 0 {
		return nil, 0
	}
	scale := 1.0 / float64(segments)
	for k := range psd {
		psd[k] *= scale
	}
	return psd, n
}

func hannWindow(n int) []float64 {
	out := make([]float64, n)
	if n == 1 {
		out[0] = 1
		return out
	}
	for i := range out {
		out[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return out
}

func prevPowerOfTwo(n int) int {
	if n < 1 {
		return 0
	}
	p := 1
	for p<<1 <= n {
		p <<= 1
	}
	return p
}

// dominantSpectralPeak returns the frequency and power of the strongest
// non-DC bin. effectiveSegSize must be the FFT size welchPSD actually used.
func dominantSpectralPeak(psd []float64, fs float64, effectiveSegSize int) (freqHz, power float64) {
	bestBin := -1
	bestPower := 0.0
	half := len(psd) / 2 // bins above N/2 are the conjugate mirror
	for k := 1; k <= half; k++ {
		if psd[k] > bestPower {
			bestPower = psd[k]
			bestBin = k
		}
	}
	if bestBin < 0 {
		return 0, 0
	}
	binWidth := fs / float64(effectiveSegSize)
	return float64(bestBin) * binWidth, bestPower
}

// spectralBands sums Welch PSD power into fixed bands: <100, 100-300, 300-600,
// >600 Hz. The PSD is a power density (unit-amplitude sine at bin center yields
// ~A^2/2 peak density), so each bin contributes density x binWidth to report
// approximate band power; DC and the conjugate mirror above N/2 are excluded.
func spectralBands(psd []float64, fs float64, effectiveSegSize int) SpectralBands {
	var bands SpectralBands
	binWidth := fs / float64(effectiveSegSize)
	for k, power := range psd {
		if k == 0 || k > len(psd)/2 {
			continue // exclude DC and the conjugate mirror above N/2
		}
		freq := float64(k) * binWidth
		switch {
		case freq < 100:
			bands.Below100Hz += power * binWidth
		case freq < 300:
			bands.Hz100To300 += power * binWidth
		case freq < 600:
			bands.Hz300To600 += power * binWidth
		default:
			bands.Above600Hz += power * binWidth
		}
	}
	return bands
}

func analyzeGyroNoise(gyros map[int]axisSeries, rateHz float64, opts AnalysisOptions) (*GyroNoiseAnalysis, string, error) {
	if len(gyros) == 0 {
		return nil, "", nil
	}
	if rateHz <= 0 {
		return nil, "unknown axis rate: neither looptime header nor usable time stream present; gyro noise skipped", nil
	}
	segment := opts.SegmentSize
	if segment <= 0 {
		segment = defaultWelchSegment
	}
	if segment&(segment-1) != 0 {
		return nil, "", fmt.Errorf("segment size %d is not a power of two", segment)
	}
	analysis := &GyroNoiseAnalysis{PerAxis: map[string]GyroNoiseAxis{}}
	indices := sortedAxisIndices(gyros)
	for _, idx := range indices {
		axis := gyros[idx]
		psd, effSeg := welchPSD(axis.values, rateHz, segment)
		if len(psd) == 0 {
			continue
		}
		peakHz, peakPower := dominantSpectralPeak(psd, rateHz, effSeg)
		analysis.PerAxis[axis.field] = GyroNoiseAxis{
			Samples:        len(axis.values),
			DominantPeakHz: peakHz,
			PeakPower:      peakPower,
			Bands:          spectralBands(psd, rateHz, effSeg),
		}
	}
	if len(analysis.PerAxis) == 0 {
		return nil, "", nil
	}
	return analysis, "", nil
}

func sortedAxisIndices(axes map[int]axisSeries) []int {
	keys := make([]int, 0, len(axes))
	for k := range axes {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// --- Step response ------------------------------------------------------------

type stepEdge struct {
	sampleIndex int
	delta       float64
}

// findStepEdges locates setpoint transitions larger than
// stepDeltaFraction * max|setpoint| separated by at least one response window.
func findStepEdges(setpoint []float64, deltaFraction float64) []stepEdge {
	if len(setpoint) < 3 {
		return nil
	}
	if deltaFraction <= 0 {
		deltaFraction = defaultStepDeltaFrac
	}
	maxAbs := 0.0
	for _, v := range setpoint {
		if math.Abs(v) > maxAbs {
			maxAbs = math.Abs(v)
		}
	}
	threshold := deltaFraction * maxAbs
	if threshold <= 0 {
		return nil
	}
	refractory := stepResponseWindow
	var edges []stepEdge
	lastEdge := -refractory - 1
	for i := 1; i < len(setpoint); i++ {
		delta := setpoint[i] - setpoint[i-1]
		if i-lastEdge <= refractory {
			continue
		}
		if math.Abs(delta) >= threshold {
			edges = append(edges, stepEdge{sampleIndex: i, delta: delta})
			lastEdge = i
		}
	}
	return edges
}

// analyzeSingleStep measures overshoot% of the gyro response following one
// setpoint edge. The settle value is the sample index (relative to the edge)
// at which the response first enters and stays inside a +/-10% band; callers
// convert it to milliseconds using the axis frame rate. Returns ok=false when
// the response does not settle inside the observation window.
func analyzeSingleStep(gyro []float64, edge stepEdge) (overshootPercent, settleSamples float64, ok bool) {
	preStart := edge.sampleIndex - 16
	if preStart < 0 {
		preStart = 0
	}
	if edge.sampleIndex-preStart < 4 || edge.sampleIndex >= len(gyro) {
		return 0, 0, false
	}
	baseline := mean(gyro[preStart:edge.sampleIndex])

	end := edge.sampleIndex + stepResponseWindow
	if end > len(gyro) {
		end = len(gyro)
	}
	response := make([]float64, 0, end-edge.sampleIndex)
	for i := edge.sampleIndex; i < end; i++ {
		response = append(response, gyro[i]-baseline)
	}
	if len(response) < 16 {
		return 0, 0, false
	}
	tail := response[len(response)*3/4:]
	settled := mean(tail)
	target := math.Abs(edge.delta)
	if target <= 0 {
		return 0, 0, false
	}

	peak := response[0]
	trough := response[0]
	for _, v := range response {
		if v > peak {
			peak = v
		}
		if v < trough {
			trough = v
		}
	}
	excess := peak - settled
	if edge.delta < 0 {
		excess = settled - trough
	}
	overshoot := excess / target * 100
	if overshoot < 0 {
		overshoot = 0
	}

	band := 0.1 * target
	settleIdx := -1
	for i, v := range response {
		if math.Abs(v-settled) <= band {
			stays := true
			for _, later := range response[i:] {
				if math.Abs(later-settled) > band {
					stays = false
					break
				}
			}
			if stays {
				settleIdx = i
				break
			}
		}
	}
	if settleIdx < 0 {
		return overshoot, 0, false
	}
	// settleIdx is a sample offset from the edge; analyzeStepResponse converts
	// it to milliseconds using the axis frame rate.
	return overshoot, float64(settleIdx), true
}

// analyzeStepResponse correlates gyro responses with setpoint edges and
// reports per-axis and overall medians. Settle times are converted from
// samples to milliseconds using rateHz (ms = samples / rate * 1000). When
// rateHz is unknown, settle fields stay nil and a caveat is returned so the
// caller can explain that settle times are unavailable in physical units.
func analyzeStepResponse(setpoints, gyros map[int]axisSeries, rateHz float64, opts AnalysisOptions) (*StepResponseAnalysis, string) {
	if len(setpoints) == 0 || len(gyros) == 0 {
		return nil, ""
	}
	minEdges := minStepEdgeCount(opts)
	analysis := &StepResponseAnalysis{PerAxis: map[string]StepResponseAxis{}}
	var allOvershoots, allSettlesMs []float64
	totalEdges := 0
	for _, idx := range sortedAxisIndices(setpoints) {
		gyroSeries, hasGyro := gyros[idx]
		if !hasGyro {
			continue
		}
		setpointSeries := setpoints[idx].values
		edges := findStepEdges(setpointSeries, opts.StepDeltaFraction)
		totalEdges += len(edges)
		var overshoots, settlesMs []float64
		for _, edge := range edges {
			overshoot, settleSamples, ok := analyzeSingleStep(gyroSeries.values, edge)
			if !ok {
				continue
			}
			overshoots = append(overshoots, overshoot)
			allOvershoots = append(allOvershoots, overshoot)
			if rateHz > 0 {
				settleMs := settleSamples / rateHz * 1000
				settlesMs = append(settlesMs, settleMs)
				allSettlesMs = append(allSettlesMs, settleMs)
			}
		}
		axisResult := StepResponseAxis{}
		if len(overshoots) > 0 {
			medianOvershoot := medianFloat(overshoots)
			axisResult.OvershootPercent = &medianOvershoot
		}
		if len(settlesMs) > 0 {
			medianSettle := medianFloat(settlesMs)
			axisResult.SettleMs = &medianSettle
		}
		analysis.PerAxis[gyroSeries.field] = axisResult
	}
	if totalEdges < minEdges || len(allOvershoots) == 0 {
		return nil, ""
	}
	medianOvershoot := medianFloat(allOvershoots)
	analysis.EdgesAnalyzed = len(allOvershoots)
	analysis.MedianOvershootPercent = &medianOvershoot
	if len(allSettlesMs) > 0 {
		medianSettle := medianFloat(allSettlesMs)
		analysis.MedianSettleMs = &medianSettle
	}
	caveat := ""
	if rateHz <= 0 {
		caveat = "unknown axis frame rate: settle times reported in samples; settle_ms fields omitted"
	}
	return analysis, caveat
}

func minStepEdgeCount(opts AnalysisOptions) int {
	if opts.MinStepEdges > 0 {
		return opts.MinStepEdges
	}
	return defaultMinStepEdges
}

// --- Motor balance --------------------------------------------------------------

// throttleBuckets splits observed throttle values into quartile buckets.
func throttleBuckets(throttle []float64) [][]int {
	min, max := throttle[0], throttle[0]
	for _, t := range throttle[1:] {
		if t < min {
			min = t
		}
		if t > max {
			max = t
		}
	}
	width := (max - min) / 4
	if width <= 0 {
		return [][]int{seqInts(len(throttle))}
	}
	buckets := make([][]int, 4)
	for i, t := range throttle {
		band := int((t - min) / width)
		if band > 3 {
			band = 3
		}
		buckets[band] = append(buckets[band], i)
	}
	return buckets
}

func seqInts(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func analyzeMotorBalance(motors []axisSeries, opts AnalysisOptions) *MotorBalanceAnalysis {
	if len(motors) < 2 {
		return nil
	}
	sorted := append([]axisSeries(nil), motors...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].index < sorted[j].index })
	count := len(sorted[0].values)
	for _, m := range sorted {
		if len(m.values) != count {
			return nil
		}
	}

	stats := make([]MotorStats, len(sorted))
	throttle := make([]float64, count)
	for si, m := range sorted {
		meanVal, std := meanStd(m.values)
		stats[si] = MotorStats{Index: m.index, Mean: meanVal, StdDev: std, Samples: count}
		for i, v := range m.values {
			throttle[i] += v / float64(len(sorted))
		}
	}

	threshold := opts.MotorSpreadThresholdPercent
	if threshold <= 0 {
		threshold = defaultMotorSpreadLimit
	}

	// Spread of per-motor means within each throttle bucket.
	maxSpread := 0.0
	outlierIndex := -1
	evaluatedBands := 0
	for _, bucket := range throttleBuckets(throttle) {
		if len(bucket) < 8 {
			continue // not enough samples in this band to compare means
		}
		means := make([]float64, len(sorted))
		for si, m := range sorted {
			sum := 0.0
			for _, i := range bucket {
				sum += m.values[i]
			}
			means[si] = sum / float64(len(bucket))
		}
		globalMean := mean(means)
		if globalMean <= 0 {
			continue
		}
		worstIdx, worstDev := 0, 0.0
		minMean, maxMean := means[0], means[0]
		for si, m := range means {
			dev := math.Abs(m-globalMean) / globalMean * 100
			if dev > worstDev {
				worstDev = dev
				worstIdx = si
			}
			if m < minMean {
				minMean = m
			}
			if m > maxMean {
				maxMean = m
			}
		}
		spread := (maxMean - minMean) / globalMean * 100
		if spread > maxSpread {
			maxSpread = spread
			outlierIndex = worstIdx
		}
		evaluatedBands++
	}

	analysis := &MotorBalanceAnalysis{Motors: stats}
	if evaluatedBands > 0 {
		spreadCopy := maxSpread
		analysis.MaxSpreadPercent = &spreadCopy
		if maxSpread > threshold {
			outlier := sorted[outlierIndex].index
			analysis.OutlierMotorIndex = &outlier
		}
	}
	return analysis
}

// --- small numeric helpers ------------------------------------------------------

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func meanStd(values []float64) (float64, float64) {
	m := mean(values)
	if len(values) < 2 {
		return m, 0
	}
	variance := 0.0
	for _, v := range values {
		d := v - m
		variance += d * d
	}
	variance /= float64(len(values) - 1)
	return m, math.Sqrt(variance)
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
