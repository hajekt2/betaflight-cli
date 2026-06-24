package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestCapabilityMetadataUsesKnownOperations(t *testing.T) {
	registry := capabilityMetadataRegistry()
	allowed := map[string]bool{
		"offline":                         true,
		"offline_or_read_only_probe":      true,
		"offline_dangerous_plan":          true,
		"read_only":                       true,
		"read_only_or_write_or_dangerous": true,
		"text_output":                     true,
		"write":                           true,
		"write_when_apply_is_set":         true,
		"plan":                            true,
		"plan_or_write":                   true,
		"plan_or_dangerous_execute":       true,
		"dangerous":                       true,
	}

	for path, meta := range registry {
		if meta.Operation == "" {
			t.Fatalf("capability %q missing operation", path)
		}
		if !allowed[meta.Operation] {
			t.Fatalf("capability %q has unknown operation %q", path, meta.Operation)
		}
	}
}

func TestCapabilityMetadataOfflineCommandClassifications(t *testing.T) {
	registry := capabilityMetadataRegistry()
	for _, tt := range []struct {
		path               string
		requiresConnection bool
		operation          string
	}{
		{"betaflight-cli schema", false, "offline"},
		{"betaflight-cli capabilities coverage", false, "offline"},
		{"betaflight-cli cli diff", true, "read_only"},
		{"betaflight-cli cli dump", true, "read_only"},
		{"betaflight-cli telemetry", true, "read_only"},
		{"betaflight-cli msp list", false, "offline"},
		{"betaflight-cli msp metadata", false, "offline"},
		{"betaflight-cli msp request", true, "read_only_or_write_or_dangerous"},
	} {
		meta, ok := registry[tt.path]
		if !ok {
			t.Fatalf("missing metadata for %q", tt.path)
		}
		if meta.RequiresConnection != tt.requiresConnection {
			t.Fatalf("metadata %q requires_connection = %v, want %v", tt.path, meta.RequiresConnection, tt.requiresConnection)
		}
		if meta.Operation != tt.operation {
			t.Fatalf("metadata %q operation = %q, want %q", tt.path, meta.Operation, tt.operation)
		}
	}
}

func TestCapabilityMetadataOperationSafetyContracts(t *testing.T) {
	registry := capabilityMetadataRegistry()
	yesRequired := map[string]bool{
		"write":                     true,
		"plan_or_write":             true,
		"plan_or_dangerous_execute": true,
		"dangerous":                 true,
		"write_when_apply_is_set":   true,
	}

	offlinedOperations := map[string]bool{
		"offline":                    true,
		"offline_or_read_only_probe": true,
		"offline_dangerous_plan":     true,
		"plan":                       true,
	}

	for path, meta := range registry {
		if yesRequired[meta.Operation] && !strings.Contains(meta.Confirmation, "--yes") {
			t.Fatalf("capability %q operation %q should require confirmation, got %q", path, meta.Operation, meta.Confirmation)
		}
		if offlinedOperations[meta.Operation] && meta.RequiresConnection {
			t.Fatalf("capability %q is operation %q but requires_connection=%v", path, meta.Operation, meta.RequiresConnection)
		}
	}
}

func TestCapabilityMetadataGenericSetCommandsAreWritePlans(t *testing.T) {
	registry := capabilityMetadataRegistry()
	for _, path := range []string{
		"betaflight-cli pid set",
		"betaflight-cli rates set",
		"betaflight-cli filters set",
		"betaflight-cli receiver set",
		"betaflight-cli vtx set",
		"betaflight-cli osd set",
		"betaflight-cli gps set",
		"betaflight-cli battery set",
		"betaflight-cli failsafe set",
	} {
		meta, ok := registry[path]
		if !ok {
			t.Fatalf("missing metadata for %q", path)
		}
		if meta.Operation != "plan_or_write" || meta.OutputRoot != "change_plan" || !strings.Contains(meta.Confirmation, "--yes") {
			t.Fatalf("metadata %q = %+v, want plan_or_write change_plan with --yes confirmation", path, meta)
		}
	}
}

func TestCapabilitiesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"capabilities"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if called {
		t.Fatal("connector was called for capabilities")
	}
	data := env.Data.(map[string]any)
	capabilities := data["capabilities"].(map[string]any)
	safety := capabilities["safety_model"].(map[string]any)
	if safety["auto_port_writes_default"] != true || safety["auto_port_writes_require_opt_in"] != false {
		t.Fatalf("safety_model auto-port fields = %+v", safety)
	}
	commands := capabilities["commands"].([]any)
	if len(commands) < 100 {
		t.Fatalf("commands = %d", len(commands))
	}
	byCommand := map[string]map[string]any{}
	for _, item := range commands {
		command := item.(map[string]any)
		byCommand[command["command"].(string)] = command
	}
	save := byCommand["betaflight-cli save"]
	if save["operation"] != "dangerous" || save["confirmation"] != "--yes" || save["requires_connection"] != true || save["runnable"] != true {
		t.Fatalf("save capability = %+v", save)
	}
	firmwareFlash := byCommand["betaflight-cli firmware flash"]
	if firmwareFlash["operation"] != "plan_or_dangerous_execute" || firmwareFlash["requires_connection"] != false || !strings.Contains(firmwareFlash["confirmation"].(string), "--execute") || firmwareFlash["runnable"] != true {
		t.Fatalf("firmware flash capability = %+v", firmwareFlash)
	}
	configurationDiff := byCommand["betaflight-cli configuration diff"]
	if configurationDiff["operation"] != "read_only" || configurationDiff["requires_connection"] != true || configurationDiff["confirmation"] != "none" || configurationDiff["runnable"] != true {
		t.Fatalf("configuration diff capability = %+v", configurationDiff)
	}
	configurationPlan := byCommand["betaflight-cli configuration plan"]
	if configurationPlan["operation"] != "offline" || configurationPlan["requires_connection"] != false || configurationPlan["confirmation"] != "none" || configurationPlan["runnable"] != true {
		t.Fatalf("configuration plan capability = %+v", configurationPlan)
	}
	configurationApply := byCommand["betaflight-cli configuration apply"]
	if configurationApply["operation"] != "write" || configurationApply["requires_connection"] != true || configurationApply["confirmation"] != "--yes" || configurationApply["runnable"] != true {
		t.Fatalf("configuration apply capability = %+v", configurationApply)
	}
	settingsDiff := byCommand["betaflight-cli settings diff"]
	if settingsDiff["operation"] != "read_only" || settingsDiff["requires_connection"] != true || settingsDiff["confirmation"] != "none" || settingsDiff["runnable"] != true {
		t.Fatalf("settings diff capability = %+v", settingsDiff)
	}
	textSetJSON := byCommand["betaflight-cli text set-json"]
	if textSetJSON["operation"] != "write" || textSetJSON["confirmation"] != "--yes" || textSetJSON["requires_connection"] != true || textSetJSON["output_root"] != "text" || textSetJSON["input"] == "" || textSetJSON["runnable"] != true {
		t.Fatalf("text set json capability = %+v", textSetJSON)
	}
	rtcSetJSON := byCommand["betaflight-cli rtc set-json"]
	if rtcSetJSON["operation"] != "write" || rtcSetJSON["confirmation"] != "--yes" || rtcSetJSON["requires_connection"] != true || rtcSetJSON["output_root"] != "rtc" || rtcSetJSON["input"] == "" || rtcSetJSON["runnable"] != true {
		t.Fatalf("rtc set json capability = %+v", rtcSetJSON)
	}
	debugTrimJSON := byCommand["betaflight-cli debug set-accelerometer-trim-json"]
	if debugTrimJSON["operation"] != "write" || debugTrimJSON["confirmation"] != "--yes" || debugTrimJSON["requires_connection"] != true || debugTrimJSON["output_root"] != "accelerometer_trim" || debugTrimJSON["input"] == "" || debugTrimJSON["runnable"] != true {
		t.Fatalf("debug trim json capability = %+v", debugTrimJSON)
	}
	featureMask := byCommand["betaflight-cli features set-mask"]
	if featureMask["operation"] != "write" || featureMask["confirmation"] != "--yes" || featureMask["requires_connection"] != true || featureMask["output_root"] != "feature_mask" || featureMask["runnable"] != true {
		t.Fatalf("feature mask capability = %+v", featureMask)
	}
	featureSetJSON := byCommand["betaflight-cli features set-json"]
	if featureSetJSON["operation"] != "plan_or_write" || featureSetJSON["confirmation"] != "--yes with --apply" || featureSetJSON["requires_connection"] != true || featureSetJSON["output_root"] != "change_plan" || featureSetJSON["input"] == "" || featureSetJSON["runnable"] != true {
		t.Fatalf("feature set json capability = %+v", featureSetJSON)
	}
	beeperEnable := byCommand["betaflight-cli beeper enable"]
	if beeperEnable["operation"] != "plan_or_write" || beeperEnable["requires_connection"] != true || beeperEnable["confirmation"] != "--yes with --apply or --save" || beeperEnable["runnable"] != true {
		t.Fatalf("beeper enable capability = %+v", beeperEnable)
	}
	beeperDisable := byCommand["betaflight-cli beeper disable"]
	if beeperDisable["operation"] != "plan_or_write" || beeperDisable["requires_connection"] != true || beeperDisable["confirmation"] != "--yes with --apply or --save" || beeperDisable["runnable"] != true {
		t.Fatalf("beeper disable capability = %+v", beeperDisable)
	}
	beeperSetJSON := byCommand["betaflight-cli beeper set-json"]
	if beeperSetJSON["operation"] != "plan_or_write" || beeperSetJSON["requires_connection"] != true || beeperSetJSON["confirmation"] != "--yes with --apply or --save" || beeperSetJSON["output_root"] != "change_plan" || beeperSetJSON["input"] == "" || beeperSetJSON["runnable"] != true {
		t.Fatalf("beeper set json capability = %+v", beeperSetJSON)
	}
	beeperConfig := byCommand["betaflight-cli beeper set-config"]
	if beeperConfig["operation"] != "write" || beeperConfig["confirmation"] != "--yes" || beeperConfig["requires_connection"] != true || beeperConfig["output_root"] != "beeper_config" || beeperConfig["runnable"] != true {
		t.Fatalf("beeper config capability = %+v", beeperConfig)
	}
	beeperConfigJSON := byCommand["betaflight-cli beeper set-config-json"]
	if beeperConfigJSON["operation"] != "write" || beeperConfigJSON["confirmation"] != "--yes" || beeperConfigJSON["requires_connection"] != true || beeperConfigJSON["output_root"] != "beeper_config" || beeperConfigJSON["input"] == "" || beeperConfigJSON["runnable"] != true {
		t.Fatalf("beeper config json capability = %+v", beeperConfigJSON)
	}
	transponderConfigJSON := byCommand["betaflight-cli transponder set-config-json"]
	if transponderConfigJSON["operation"] != "write" || transponderConfigJSON["confirmation"] != "--yes" || transponderConfigJSON["requires_connection"] != true || transponderConfigJSON["output_root"] != "transponder_config" || transponderConfigJSON["input"] == "" || transponderConfigJSON["runnable"] != true {
		t.Fatalf("transponder config json capability = %+v", transponderConfigJSON)
	}
	cliInteractive := byCommand["betaflight-cli cli interactive"]
	if cliInteractive["operation"] != "dangerous" || cliInteractive["confirmation"] != "--yes" || cliInteractive["requires_connection"] != true || cliInteractive["runnable"] != true {
		t.Fatalf("cli interactive capability = %+v", cliInteractive)
	}
	sensorCalibration := byCommand["betaflight-cli sensors calibrate-accelerometer"]
	if sensorCalibration["operation"] != "dangerous" || sensorCalibration["confirmation"] != "--yes" || sensorCalibration["requires_connection"] != true || sensorCalibration["output_root"] != "sensor_calibration" || sensorCalibration["runnable"] != true {
		t.Fatalf("sensor calibration capability = %+v", sensorCalibration)
	}
	rtcSet := byCommand["betaflight-cli rtc set"]
	if rtcSet["operation"] != "write" || rtcSet["confirmation"] != "--yes" || rtcSet["requires_connection"] != true || rtcSet["output_root"] != "rtc" || rtcSet["runnable"] != true {
		t.Fatalf("rtc set capability = %+v", rtcSet)
	}
	accTrim := byCommand["betaflight-cli debug set-accelerometer-trim"]
	if accTrim["operation"] != "write" || accTrim["confirmation"] != "--yes" || accTrim["requires_connection"] != true || accTrim["output_root"] != "accelerometer_trim" || accTrim["runnable"] != true {
		t.Fatalf("accelerometer trim capability = %+v", accTrim)
	}
	sensorConfig := byCommand["betaflight-cli sensors set-config"]
	if sensorConfig["operation"] != "write" || sensorConfig["confirmation"] != "--yes" || sensorConfig["requires_connection"] != true || sensorConfig["output_root"] != "sensor_config" || sensorConfig["runnable"] != true {
		t.Fatalf("sensor config capability = %+v", sensorConfig)
	}
	sensorConfigJSON := byCommand["betaflight-cli sensors set-config-json"]
	if sensorConfigJSON["operation"] != "write" || sensorConfigJSON["confirmation"] != "--yes" || sensorConfigJSON["requires_connection"] != true || sensorConfigJSON["output_root"] != "sensor_config" || sensorConfigJSON["runnable"] != true {
		t.Fatalf("sensor config JSON capability = %+v", sensorConfigJSON)
	}
	sensorAlignment := byCommand["betaflight-cli sensors set-alignment"]
	if sensorAlignment["operation"] != "write" || sensorAlignment["confirmation"] != "--yes" || sensorAlignment["requires_connection"] != true || sensorAlignment["output_root"] != "sensor_alignment" || sensorAlignment["runnable"] != true {
		t.Fatalf("sensor alignment capability = %+v", sensorAlignment)
	}
	sensorAlignmentJSON := byCommand["betaflight-cli sensors set-alignment-json"]
	if sensorAlignmentJSON["operation"] != "write" || sensorAlignmentJSON["confirmation"] != "--yes" || sensorAlignmentJSON["requires_connection"] != true || sensorAlignmentJSON["output_root"] != "sensor_alignment" || sensorAlignmentJSON["runnable"] != true {
		t.Fatalf("sensor alignment JSON capability = %+v", sensorAlignmentJSON)
	}
	compassConfig := byCommand["betaflight-cli sensors set-compass-declination"]
	if compassConfig["operation"] != "write" || compassConfig["confirmation"] != "--yes" || compassConfig["requires_connection"] != true || compassConfig["output_root"] != "compass_config" || compassConfig["runnable"] != true {
		t.Fatalf("compass config capability = %+v", compassConfig)
	}
	compassConfigJSON := byCommand["betaflight-cli sensors set-compass-json"]
	if compassConfigJSON["operation"] != "write" || compassConfigJSON["confirmation"] != "--yes" || compassConfigJSON["requires_connection"] != true || compassConfigJSON["output_root"] != "compass_config" || compassConfigJSON["runnable"] != true {
		t.Fatalf("compass config JSON capability = %+v", compassConfigJSON)
	}
	profileCopy := byCommand["betaflight-cli profiles copy"]
	if profileCopy["operation"] != "write" || profileCopy["confirmation"] != "--yes" || profileCopy["requires_connection"] != true || profileCopy["output_root"] != "profile_copy" || profileCopy["runnable"] != true {
		t.Fatalf("profile copy capability = %+v", profileCopy)
	}
	textSet := byCommand["betaflight-cli text set"]
	if textSet["operation"] != "write" || textSet["confirmation"] != "--yes" || textSet["requires_connection"] != true || textSet["output_root"] != "text" || textSet["runnable"] != true {
		t.Fatalf("text set capability = %+v", textSet)
	}
	boardAlignment := byCommand["betaflight-cli failsafe set-board-alignment"]
	if boardAlignment["operation"] != "write" || boardAlignment["confirmation"] != "--yes" || boardAlignment["requires_connection"] != true || boardAlignment["output_root"] != "board_alignment" || boardAlignment["runnable"] != true {
		t.Fatalf("board alignment capability = %+v", boardAlignment)
	}
	boardAlignmentJSON := byCommand["betaflight-cli failsafe set-board-alignment-json"]
	if boardAlignmentJSON["operation"] != "write" || boardAlignmentJSON["confirmation"] != "--yes" || boardAlignmentJSON["requires_connection"] != true || boardAlignmentJSON["output_root"] != "board_alignment" || boardAlignmentJSON["input"] == "" || boardAlignmentJSON["runnable"] != true {
		t.Fatalf("board alignment JSON capability = %+v", boardAlignmentJSON)
	}
	receiverConfig := byCommand["betaflight-cli receiver set-config-json"]
	if receiverConfig["operation"] != "write" || receiverConfig["confirmation"] != "--yes" || receiverConfig["requires_connection"] != true || receiverConfig["output_root"] != "receiver_config" || receiverConfig["runnable"] != true {
		t.Fatalf("receiver config capability = %+v", receiverConfig)
	}
	rssiChannel := byCommand["betaflight-cli receiver set-rssi-channel"]
	if rssiChannel["operation"] != "write" || rssiChannel["confirmation"] != "--yes" || rssiChannel["requires_connection"] != true || rssiChannel["output_root"] != "rssi_channel" || rssiChannel["runnable"] != true {
		t.Fatalf("rssi channel capability = %+v", rssiChannel)
	}
	rssiChannelJSON := byCommand["betaflight-cli receiver set-rssi-channel-json"]
	if rssiChannelJSON["operation"] != "write" || rssiChannelJSON["confirmation"] != "--yes" || rssiChannelJSON["requires_connection"] != true || rssiChannelJSON["output_root"] != "rssi_channel" || rssiChannelJSON["input"] == "" || rssiChannelJSON["runnable"] != true {
		t.Fatalf("rssi channel JSON capability = %+v", rssiChannelJSON)
	}
	rxFail := byCommand["betaflight-cli receiver set-rxfail"]
	if rxFail["operation"] != "write" || rxFail["confirmation"] != "--yes" || rxFail["requires_connection"] != true || rxFail["output_root"] != "rx_fail" || rxFail["runnable"] != true {
		t.Fatalf("rx fail capability = %+v", rxFail)
	}
	rxFailTable := byCommand["betaflight-cli receiver set-rxfail-json"]
	if rxFailTable["operation"] != "write" || rxFailTable["confirmation"] != "--yes" || rxFailTable["requires_connection"] != true || rxFailTable["output_root"] != "rx_fail_table" || rxFailTable["runnable"] != true {
		t.Fatalf("rx fail table capability = %+v", rxFailTable)
	}
	rcMap := byCommand["betaflight-cli receiver set-map"]
	if rcMap["operation"] != "write" || rcMap["confirmation"] != "--yes" || rcMap["requires_connection"] != true || rcMap["output_root"] != "rc_map" || rcMap["runnable"] != true {
		t.Fatalf("rc map capability = %+v", rcMap)
	}
	rcMapJSON := byCommand["betaflight-cli receiver set-map-json"]
	if rcMapJSON["operation"] != "write" || rcMapJSON["confirmation"] != "--yes" || rcMapJSON["requires_connection"] != true || rcMapJSON["output_root"] != "rc_map" || rcMapJSON["runnable"] != true {
		t.Fatalf("rc map JSON capability = %+v", rcMapJSON)
	}
	rcDeadband := byCommand["betaflight-cli receiver set-deadband"]
	if rcDeadband["operation"] != "write" || rcDeadband["confirmation"] != "--yes" || rcDeadband["requires_connection"] != true || rcDeadband["output_root"] != "rc_deadband" || rcDeadband["runnable"] != true {
		t.Fatalf("rc deadband capability = %+v", rcDeadband)
	}
	rcDeadbandJSON := byCommand["betaflight-cli receiver set-deadband-json"]
	if rcDeadbandJSON["operation"] != "write" || rcDeadbandJSON["confirmation"] != "--yes" || rcDeadbandJSON["requires_connection"] != true || rcDeadbandJSON["output_root"] != "rc_deadband" || rcDeadbandJSON["runnable"] != true {
		t.Fatalf("rc deadband JSON capability = %+v", rcDeadbandJSON)
	}
	rxrangeJSON := byCommand["betaflight-cli rxrange set-json"]
	if rxrangeJSON["operation"] != "plan_or_write" || rxrangeJSON["confirmation"] != "--yes with --apply or --save" || rxrangeJSON["requires_connection"] != true || rxrangeJSON["output_root"] != "change_plan" || rxrangeJSON["input"] == "" || rxrangeJSON["runnable"] != true {
		t.Fatalf("rxrange JSON capability = %+v", rxrangeJSON)
	}
	gpsConfig := byCommand["betaflight-cli gps set-config"]
	if gpsConfig["operation"] != "write" || gpsConfig["confirmation"] != "--yes" || gpsConfig["requires_connection"] != true || gpsConfig["output_root"] != "gps_config" || gpsConfig["runnable"] != true {
		t.Fatalf("gps config capability = %+v", gpsConfig)
	}
	gpsConfigJSON := byCommand["betaflight-cli gps set-config-json"]
	if gpsConfigJSON["operation"] != "write" || gpsConfigJSON["confirmation"] != "--yes" || gpsConfigJSON["requires_connection"] != true || gpsConfigJSON["output_root"] != "gps_config" || gpsConfigJSON["runnable"] != true {
		t.Fatalf("gps config JSON capability = %+v", gpsConfigJSON)
	}
	gpsRescue := byCommand["betaflight-cli gps set-rescue"]
	if gpsRescue["operation"] != "write" || gpsRescue["confirmation"] != "--yes" || gpsRescue["requires_connection"] != true || gpsRescue["output_root"] != "gps_rescue" || gpsRescue["runnable"] != true {
		t.Fatalf("gps rescue capability = %+v", gpsRescue)
	}
	gpsRescueJSON := byCommand["betaflight-cli gps set-rescue-json"]
	if gpsRescueJSON["operation"] != "write" || gpsRescueJSON["confirmation"] != "--yes" || gpsRescueJSON["requires_connection"] != true || gpsRescueJSON["output_root"] != "gps_rescue" || gpsRescueJSON["runnable"] != true {
		t.Fatalf("gps rescue JSON capability = %+v", gpsRescueJSON)
	}
	gpsRescuePIDs := byCommand["betaflight-cli gps set-rescue-pids"]
	if gpsRescuePIDs["operation"] != "write" || gpsRescuePIDs["confirmation"] != "--yes" || gpsRescuePIDs["requires_connection"] != true || gpsRescuePIDs["output_root"] != "gps_rescue_pids" || gpsRescuePIDs["runnable"] != true {
		t.Fatalf("gps rescue pids capability = %+v", gpsRescuePIDs)
	}
	gpsRescuePIDsJSON := byCommand["betaflight-cli gps set-rescue-pids-json"]
	if gpsRescuePIDsJSON["operation"] != "write" || gpsRescuePIDsJSON["confirmation"] != "--yes" || gpsRescuePIDsJSON["requires_connection"] != true || gpsRescuePIDsJSON["output_root"] != "gps_rescue_pids" || gpsRescuePIDsJSON["runnable"] != true {
		t.Fatalf("gps rescue pids JSON capability = %+v", gpsRescuePIDsJSON)
	}
	armingConfig := byCommand["betaflight-cli failsafe set-arming-json"]
	if armingConfig["operation"] != "write" || armingConfig["confirmation"] != "--yes" || armingConfig["requires_connection"] != true || armingConfig["output_root"] != "arming_config" || armingConfig["runnable"] != true {
		t.Fatalf("arming config capability = %+v", armingConfig)
	}
	failsafeConfig := byCommand["betaflight-cli failsafe set-config-json"]
	if failsafeConfig["operation"] != "write" || failsafeConfig["confirmation"] != "--yes" || failsafeConfig["requires_connection"] != true || failsafeConfig["output_root"] != "failsafe_config" || failsafeConfig["runnable"] != true {
		t.Fatalf("failsafe config capability = %+v", failsafeConfig)
	}
	pidGains := byCommand["betaflight-cli pid set-gains-json"]
	if pidGains["operation"] != "write" || pidGains["confirmation"] != "--yes" || pidGains["requires_connection"] != true || pidGains["output_root"] != "pid_gains" || pidGains["runnable"] != true {
		t.Fatalf("pid gains capability = %+v", pidGains)
	}
	pidAdvanced := byCommand["betaflight-cli pid set-advanced-json"]
	if pidAdvanced["operation"] != "write" || pidAdvanced["confirmation"] != "--yes" || pidAdvanced["requires_connection"] != true || pidAdvanced["output_root"] != "pid_advanced" || pidAdvanced["runnable"] != true {
		t.Fatalf("pid advanced capability = %+v", pidAdvanced)
	}
	simplifiedPreview := byCommand["betaflight-cli pid preview-simplified-json"]
	if simplifiedPreview["operation"] != "read_only" || simplifiedPreview["confirmation"] != "none" || simplifiedPreview["requires_connection"] != true || simplifiedPreview["output_root"] != "simplified_tuning_preview" || simplifiedPreview["runnable"] != true {
		t.Fatalf("simplified preview capability = %+v", simplifiedPreview)
	}
	simplifiedValidation := byCommand["betaflight-cli pid validate-simplified"]
	if simplifiedValidation["operation"] != "read_only" || simplifiedValidation["confirmation"] != "none" || simplifiedValidation["requires_connection"] != true || simplifiedValidation["output_root"] != "simplified_tuning_validation" || simplifiedValidation["runnable"] != true {
		t.Fatalf("simplified validation capability = %+v", simplifiedValidation)
	}
	simplifiedTuning := byCommand["betaflight-cli pid set-simplified-json"]
	if simplifiedTuning["operation"] != "write" || simplifiedTuning["confirmation"] != "--yes" || simplifiedTuning["requires_connection"] != true || simplifiedTuning["output_root"] != "simplified_tuning" || simplifiedTuning["runnable"] != true {
		t.Fatalf("simplified tuning capability = %+v", simplifiedTuning)
	}
	rateProfile := byCommand["betaflight-cli rates set-profile-json"]
	if rateProfile["operation"] != "write" || rateProfile["confirmation"] != "--yes" || rateProfile["requires_connection"] != true || rateProfile["output_root"] != "rate_profile" || rateProfile["runnable"] != true {
		t.Fatalf("rate profile capability = %+v", rateProfile)
	}
	advancedConfig := byCommand["betaflight-cli filters set-advanced-json"]
	if advancedConfig["operation"] != "write" || advancedConfig["confirmation"] != "--yes" || advancedConfig["requires_connection"] != true || advancedConfig["output_root"] != "advanced_config" || advancedConfig["runnable"] != true {
		t.Fatalf("advanced config capability = %+v", advancedConfig)
	}
	filterConfig := byCommand["betaflight-cli filters set-filter-json"]
	if filterConfig["operation"] != "write" || filterConfig["confirmation"] != "--yes" || filterConfig["requires_connection"] != true || filterConfig["output_root"] != "filter_config" || filterConfig["runnable"] != true {
		t.Fatalf("filter config capability = %+v", filterConfig)
	}
	blackboxConfig := byCommand["betaflight-cli blackbox set-config-json"]
	if blackboxConfig["operation"] != "write" || blackboxConfig["confirmation"] != "--yes" || blackboxConfig["requires_connection"] != true || blackboxConfig["output_root"] != "blackbox_config" || blackboxConfig["runnable"] != true {
		t.Fatalf("blackbox config capability = %+v", blackboxConfig)
	}
	mixerConfig := byCommand["betaflight-cli mixer set-config-json"]
	if mixerConfig["operation"] != "write" || mixerConfig["confirmation"] != "--yes" || mixerConfig["requires_connection"] != true || mixerConfig["output_root"] != "mixer_config" || mixerConfig["runnable"] != true {
		t.Fatalf("mixer config capability = %+v", mixerConfig)
	}
	motorConfig := byCommand["betaflight-cli motors set-config"]
	if motorConfig["operation"] != "write" || motorConfig["confirmation"] != "--yes" || motorConfig["requires_connection"] != true || motorConfig["output_root"] != "motor_config" || motorConfig["runnable"] != true {
		t.Fatalf("motor config capability = %+v", motorConfig)
	}
	motorConfigJSON := byCommand["betaflight-cli motors set-config-json"]
	if motorConfigJSON["operation"] != "write" || motorConfigJSON["confirmation"] != "--yes" || motorConfigJSON["requires_connection"] != true || motorConfigJSON["output_root"] != "motor_config" || motorConfigJSON["input"] == "" || motorConfigJSON["runnable"] != true {
		t.Fatalf("motor config json capability = %+v", motorConfigJSON)
	}
	motor3DConfig := byCommand["betaflight-cli motors set-3d-config"]
	if motor3DConfig["operation"] != "write" || motor3DConfig["confirmation"] != "--yes" || motor3DConfig["requires_connection"] != true || motor3DConfig["output_root"] != "motor_3d_config" || motor3DConfig["runnable"] != true {
		t.Fatalf("motor 3d config capability = %+v", motor3DConfig)
	}
	motor3DConfigJSON := byCommand["betaflight-cli motors set-3d-config-json"]
	if motor3DConfigJSON["operation"] != "write" || motor3DConfigJSON["confirmation"] != "--yes" || motor3DConfigJSON["requires_connection"] != true || motor3DConfigJSON["output_root"] != "motor_3d_config" || motor3DConfigJSON["input"] == "" || motor3DConfigJSON["runnable"] != true {
		t.Fatalf("motor 3d config json capability = %+v", motor3DConfigJSON)
	}
	serialConfig := byCommand["betaflight-cli serial apply-config-json"]
	if serialConfig["operation"] != "write" || serialConfig["confirmation"] != "--yes" || serialConfig["requires_connection"] != true || serialConfig["output_root"] != "serial_config" || serialConfig["runnable"] != true {
		t.Fatalf("serial config capability = %+v", serialConfig)
	}
	serialSetJSON := byCommand["betaflight-cli serial set-json"]
	if serialSetJSON["operation"] != "plan_or_write" || serialSetJSON["confirmation"] != "--yes with --apply or --save" || serialSetJSON["requires_connection"] != true || serialSetJSON["output_root"] != "change_plan" || serialSetJSON["input"] == "" || serialSetJSON["runnable"] != true {
		t.Fatalf("serial set json capability = %+v", serialSetJSON)
	}
	modeRange := byCommand["betaflight-cli modes set-range"]
	if modeRange["operation"] != "write" || modeRange["confirmation"] != "--yes" || modeRange["requires_connection"] != true || modeRange["output_root"] != "mode_range" || modeRange["runnable"] != true {
		t.Fatalf("mode range capability = %+v", modeRange)
	}
	modeRanges := byCommand["betaflight-cli modes set-json"]
	if modeRanges["operation"] != "write" || modeRanges["confirmation"] != "--yes" || modeRanges["requires_connection"] != true || modeRanges["output_root"] != "mode_ranges" || modeRanges["runnable"] != true {
		t.Fatalf("mode ranges capability = %+v", modeRanges)
	}
	ledSetJSON := byCommand["betaflight-cli leds set-json"]
	if ledSetJSON["operation"] != "plan_or_write" || ledSetJSON["confirmation"] != "--yes with --apply or --save" || ledSetJSON["requires_connection"] != true || ledSetJSON["output_root"] != "change_plan" || ledSetJSON["input"] == "" || ledSetJSON["runnable"] != true {
		t.Fatalf("led set json capability = %+v", ledSetJSON)
	}
	resourcesJSON := byCommand["betaflight-cli resources set-json"]
	if resourcesJSON["operation"] != "plan_or_write" || resourcesJSON["confirmation"] != "--yes with --apply or --save" || resourcesJSON["requires_connection"] != true || resourcesJSON["output_root"] != "change_plan" || resourcesJSON["input"] == "" || resourcesJSON["runnable"] != true {
		t.Fatalf("resources JSON capability = %+v", resourcesJSON)
	}
	ledValues := byCommand["betaflight-cli leds set-values"]
	if ledValues["operation"] != "write" || ledValues["confirmation"] != "--yes" || ledValues["requires_connection"] != true || ledValues["output_root"] != "led_values" || ledValues["runnable"] != true {
		t.Fatalf("led values capability = %+v", ledValues)
	}
	ledValuesJSON := byCommand["betaflight-cli leds set-values-json"]
	if ledValuesJSON["operation"] != "write" || ledValuesJSON["confirmation"] != "--yes" || ledValuesJSON["requires_connection"] != true || ledValuesJSON["output_root"] != "led_values" || ledValuesJSON["input"] == "" || ledValuesJSON["runnable"] != true {
		t.Fatalf("led values json capability = %+v", ledValuesJSON)
	}
	ledColors := byCommand["betaflight-cli leds set-colors-json"]
	if ledColors["operation"] != "write" || ledColors["confirmation"] != "--yes" || ledColors["requires_connection"] != true || ledColors["output_root"] != "led_colors" || ledColors["runnable"] != true {
		t.Fatalf("led colors capability = %+v", ledColors)
	}
	ledModeColor := byCommand["betaflight-cli leds set-mode-color"]
	if ledModeColor["operation"] != "write" || ledModeColor["confirmation"] != "--yes" || ledModeColor["requires_connection"] != true || ledModeColor["output_root"] != "led_mode_color" || ledModeColor["runnable"] != true {
		t.Fatalf("led mode color capability = %+v", ledModeColor)
	}
	ledModeColorJSON := byCommand["betaflight-cli leds set-mode-color-json"]
	if ledModeColorJSON["operation"] != "write" || ledModeColorJSON["confirmation"] != "--yes" || ledModeColorJSON["requires_connection"] != true || ledModeColorJSON["output_root"] != "led_mode_color" || ledModeColorJSON["input"] == "" || ledModeColorJSON["runnable"] != true {
		t.Fatalf("led mode color json capability = %+v", ledModeColorJSON)
	}
	servoConfig := byCommand["betaflight-cli servos set-config"]
	if servoConfig["operation"] != "write" || servoConfig["confirmation"] != "--yes" || servoConfig["requires_connection"] != true || servoConfig["output_root"] != "servo_config" || servoConfig["runnable"] != true {
		t.Fatalf("servo config capability = %+v", servoConfig)
	}
	servoConfigJSON := byCommand["betaflight-cli servos set-config-json"]
	if servoConfigJSON["operation"] != "write" || servoConfigJSON["confirmation"] != "--yes" || servoConfigJSON["requires_connection"] != true || servoConfigJSON["output_root"] != "servo_config" || servoConfigJSON["input"] == "" || servoConfigJSON["runnable"] != true {
		t.Fatalf("servo config json capability = %+v", servoConfigJSON)
	}
	servoTable := byCommand["betaflight-cli servos set-json"]
	if servoTable["operation"] != "write" || servoTable["confirmation"] != "--yes" || servoTable["requires_connection"] != true || servoTable["output_root"] != "servo_table" || servoTable["runnable"] != true {
		t.Fatalf("servo table capability = %+v", servoTable)
	}
	servoReverseJSON := byCommand["betaflight-cli servos reverse-json"]
	if servoReverseJSON["operation"] != "plan_or_write" || servoReverseJSON["confirmation"] != "--yes with --apply or --save" || servoReverseJSON["requires_connection"] != true || servoReverseJSON["output_root"] != "change_plan" || servoReverseJSON["input"] == "" || servoReverseJSON["runnable"] != true {
		t.Fatalf("servo reverse json capability = %+v", servoReverseJSON)
	}
	servoMixRule := byCommand["betaflight-cli servos set-mix-rule"]
	if servoMixRule["operation"] != "write" || servoMixRule["confirmation"] != "--yes" || servoMixRule["requires_connection"] != true || servoMixRule["output_root"] != "servo_mix_rule" || servoMixRule["runnable"] != true {
		t.Fatalf("servo mix rule capability = %+v", servoMixRule)
	}
	servoMixRuleJSON := byCommand["betaflight-cli servos set-mix-rule-json"]
	if servoMixRuleJSON["operation"] != "write" || servoMixRuleJSON["confirmation"] != "--yes" || servoMixRuleJSON["requires_connection"] != true || servoMixRuleJSON["output_root"] != "servo_mix_rule" || servoMixRuleJSON["input"] == "" || servoMixRuleJSON["runnable"] != true {
		t.Fatalf("servo mix rule json capability = %+v", servoMixRuleJSON)
	}
	adjustmentRange := byCommand["betaflight-cli adjustments set-range"]
	if adjustmentRange["operation"] != "write" || adjustmentRange["confirmation"] != "--yes" || adjustmentRange["requires_connection"] != true || adjustmentRange["output_root"] != "adjustment_range" || adjustmentRange["runnable"] != true {
		t.Fatalf("adjustment range capability = %+v", adjustmentRange)
	}
	adjustmentRangeJSON := byCommand["betaflight-cli adjustments set-range-json"]
	if adjustmentRangeJSON["operation"] != "write" || adjustmentRangeJSON["confirmation"] != "--yes" || adjustmentRangeJSON["requires_connection"] != true || adjustmentRangeJSON["output_root"] != "adjustment_range" || adjustmentRangeJSON["input"] == "" || adjustmentRangeJSON["runnable"] != true {
		t.Fatalf("adjustment range json capability = %+v", adjustmentRangeJSON)
	}
	adjustmentTable := byCommand["betaflight-cli adjustments set-json"]
	if adjustmentTable["operation"] != "write" || adjustmentTable["confirmation"] != "--yes" || adjustmentTable["requires_connection"] != true || adjustmentTable["output_root"] != "adjustment_table" || adjustmentTable["runnable"] != true {
		t.Fatalf("adjustment table capability = %+v", adjustmentTable)
	}
	batteryConfig := byCommand["betaflight-cli battery set-config-json"]
	if batteryConfig["operation"] != "write" || batteryConfig["confirmation"] != "--yes" || batteryConfig["requires_connection"] != true || batteryConfig["output_root"] != "battery_config" || batteryConfig["runnable"] != true {
		t.Fatalf("battery config capability = %+v", batteryConfig)
	}
	batteryProfile := byCommand["betaflight-cli battery set-profile-json"]
	if batteryProfile["operation"] != "write" || batteryProfile["confirmation"] != "--yes" || batteryProfile["requires_connection"] != true || batteryProfile["output_root"] != "battery_profile" || batteryProfile["runnable"] != true {
		t.Fatalf("battery profile capability = %+v", batteryProfile)
	}
	voltageMeter := byCommand["betaflight-cli battery set-voltage-meter"]
	if voltageMeter["operation"] != "write" || voltageMeter["confirmation"] != "--yes" || voltageMeter["requires_connection"] != true || voltageMeter["output_root"] != "voltage_meter_config" || voltageMeter["runnable"] != true {
		t.Fatalf("voltage meter capability = %+v", voltageMeter)
	}
	voltageMeterJSON := byCommand["betaflight-cli battery set-voltage-meter-json"]
	if voltageMeterJSON["operation"] != "write" || voltageMeterJSON["confirmation"] != "--yes" || voltageMeterJSON["requires_connection"] != true || voltageMeterJSON["output_root"] != "voltage_meter_config" || voltageMeterJSON["runnable"] != true {
		t.Fatalf("voltage meter JSON capability = %+v", voltageMeterJSON)
	}
	currentMeter := byCommand["betaflight-cli battery set-current-meter"]
	if currentMeter["operation"] != "write" || currentMeter["confirmation"] != "--yes" || currentMeter["requires_connection"] != true || currentMeter["output_root"] != "current_meter_config" || currentMeter["runnable"] != true {
		t.Fatalf("current meter capability = %+v", currentMeter)
	}
	currentMeterJSON := byCommand["betaflight-cli battery set-current-meter-json"]
	if currentMeterJSON["operation"] != "write" || currentMeterJSON["confirmation"] != "--yes" || currentMeterJSON["requires_connection"] != true || currentMeterJSON["output_root"] != "current_meter_config" || currentMeterJSON["runnable"] != true {
		t.Fatalf("current meter JSON capability = %+v", currentMeterJSON)
	}
	cliExec := byCommand["betaflight-cli cli exec"]
	if cliExec["operation"] != "read_only_or_write_or_dangerous" || cliExec["confirmation"] != "--yes for writes and dangerous CLI lines" || cliExec["requires_connection"] != true || cliExec["runnable"] != true {
		t.Fatalf("cli exec capability = %+v", cliExec)
	}
	vtxList := byCommand["betaflight-cli vtx list"]
	if vtxList["operation"] != "read_only" || vtxList["confirmation"] != "none" || vtxList["requires_connection"] != true || vtxList["runnable"] != true {
		t.Fatalf("vtx list capability = %+v", vtxList)
	}
	mspRequest := byCommand["betaflight-cli msp request"]
	if mspRequest["operation"] != "read_only_or_write_or_dangerous" || mspRequest["requires_connection"] != true || mspRequest["confirmation"] != "--yes for generated write-like commands and numeric commands without compiled metadata" || mspRequest["output_root"] != "msp" || mspRequest["runnable"] != true {
		t.Fatalf("msp request capability = %+v", mspRequest)
	}
	mspList := byCommand["betaflight-cli msp list"]
	if mspList["operation"] != "offline" || mspList["requires_connection"] != false || mspList["confirmation"] != "none" || mspList["output_root"] != "msp" || mspList["runnable"] != true {
		t.Fatalf("msp list capability = %+v", mspList)
	}
	mspMetadata := byCommand["betaflight-cli msp metadata"]
	if mspMetadata["operation"] != "offline" || mspMetadata["requires_connection"] != false || mspMetadata["confirmation"] != "none" || mspMetadata["output_root"] != "msp" || mspMetadata["runnable"] != true {
		t.Fatalf("msp metadata capability = %+v", mspMetadata)
	}
	cameraKeys := byCommand["betaflight-cli camera keys"]
	if cameraKeys["operation"] != "offline" || cameraKeys["requires_connection"] != false || cameraKeys["confirmation"] != "none" || cameraKeys["output_root"] != "camera_keys" || cameraKeys["runnable"] != true {
		t.Fatalf("camera keys capability = %+v", cameraKeys)
	}
	cameraPress := byCommand["betaflight-cli camera press"]
	if cameraPress["operation"] != "plan_or_write" || cameraPress["requires_connection"] != true || cameraPress["confirmation"] != "--yes" || cameraPress["output_root"] != "camera_control" || cameraPress["runnable"] != true {
		t.Fatalf("camera press capability = %+v", cameraPress)
	}
	settingsFirmwareGet := byCommand["betaflight-cli settings firmware-get"]
	if settingsFirmwareGet["operation"] != "read_only" || settingsFirmwareGet["requires_connection"] != true || settingsFirmwareGet["confirmation"] != "none" || settingsFirmwareGet["output_root"] != "firmware_setting" || settingsFirmwareGet["runnable"] != true {
		t.Fatalf("settings firmware-get capability = %+v", settingsFirmwareGet)
	}
	settingsFirmwareInfo := byCommand["betaflight-cli settings firmware-info"]
	if settingsFirmwareInfo["operation"] != "read_only" || settingsFirmwareInfo["requires_connection"] != true || settingsFirmwareInfo["confirmation"] != "none" || settingsFirmwareInfo["output_root"] != "firmware_setting_info" || settingsFirmwareInfo["runnable"] != true {
		t.Fatalf("settings firmware-info capability = %+v", settingsFirmwareInfo)
	}
	mspBatch := byCommand["betaflight-cli msp batch"]
	if mspBatch["operation"] != "read_only" || mspBatch["requires_connection"] != true || mspBatch["confirmation"] != "none" || mspBatch["output_root"] != "msp" || mspBatch["runnable"] != true {
		t.Fatalf("msp batch capability = %+v", mspBatch)
	}
	vtxTableStatus := byCommand["betaflight-cli vtxtable status"]
	if vtxTableStatus["operation"] != "read_only" || vtxTableStatus["requires_connection"] != true || vtxTableStatus["confirmation"] != "none" || vtxTableStatus["output_root"] != "vtxtable" || vtxTableStatus["runnable"] != true {
		t.Fatalf("vtxtable status capability = %+v", vtxTableStatus)
	}
	vtxDeviceStatus := byCommand["betaflight-cli vtx device-status"]
	if vtxDeviceStatus["operation"] != "read_only" || vtxDeviceStatus["requires_connection"] != true || vtxDeviceStatus["confirmation"] != "none" || vtxDeviceStatus["output_root"] != "vtx_device" || vtxDeviceStatus["runnable"] != true {
		t.Fatalf("vtx device status capability = %+v", vtxDeviceStatus)
	}
	schema := byCommand["betaflight-cli schema"]
	if schema["operation"] != "offline" || schema["requires_connection"] != false || schema["confirmation"] != "none" || schema["runnable"] != true {
		t.Fatalf("schema capability = %+v", schema)
	}
	capabilitiesCoverage := byCommand["betaflight-cli capabilities coverage"]
	if capabilitiesCoverage["operation"] != "offline" || capabilitiesCoverage["requires_connection"] != false || capabilitiesCoverage["confirmation"] != "none" || capabilitiesCoverage["output_root"] != "coverage" || capabilitiesCoverage["runnable"] != true {
		t.Fatalf("capabilities coverage capability = %+v", capabilitiesCoverage)
	}
	transponderSetProvider := byCommand["betaflight-cli transponder set-provider"]
	if transponderSetProvider["operation"] != "plan_or_write" || transponderSetProvider["requires_connection"] != true || transponderSetProvider["confirmation"] != "--yes with --apply or --save" || transponderSetProvider["runnable"] != true {
		t.Fatalf("transponder set-provider capability = %+v", transponderSetProvider)
	}
	transponderSetData := byCommand["betaflight-cli transponder set-data"]
	if transponderSetData["operation"] != "plan_or_write" || transponderSetData["requires_connection"] != true || transponderSetData["confirmation"] != "--yes with --apply or --save" || transponderSetData["runnable"] != true {
		t.Fatalf("transponder set-data capability = %+v", transponderSetData)
	}
	transponderSetJSON := byCommand["betaflight-cli transponder set-json"]
	if transponderSetJSON["operation"] != "plan_or_write" || transponderSetJSON["requires_connection"] != true || transponderSetJSON["confirmation"] != "--yes with --apply or --save" || transponderSetJSON["output_root"] != "change_plan" || transponderSetJSON["input"] == "" || transponderSetJSON["runnable"] != true {
		t.Fatalf("transponder set json capability = %+v", transponderSetJSON)
	}
	transponderConfig := byCommand["betaflight-cli transponder set-config"]
	if transponderConfig["operation"] != "write" || transponderConfig["confirmation"] != "--yes" || transponderConfig["requires_connection"] != true || transponderConfig["output_root"] != "transponder_config" || transponderConfig["runnable"] != true {
		t.Fatalf("transponder config capability = %+v", transponderConfig)
	}
	vtxConfig := byCommand["betaflight-cli vtx set-config"]
	if vtxConfig["operation"] != "write" || vtxConfig["confirmation"] != "--yes" || vtxConfig["requires_connection"] != true || vtxConfig["output_root"] != "vtx_config" || vtxConfig["runnable"] != true {
		t.Fatalf("vtx config capability = %+v", vtxConfig)
	}
	vtxConfigJSON := byCommand["betaflight-cli vtx set-config-json"]
	if vtxConfigJSON["operation"] != "write" || vtxConfigJSON["confirmation"] != "--yes" || vtxConfigJSON["requires_connection"] != true || vtxConfigJSON["output_root"] != "vtx_config" || vtxConfigJSON["input"] == "" || vtxConfigJSON["runnable"] != true {
		t.Fatalf("vtx config json capability = %+v", vtxConfigJSON)
	}
	osdCanvas := byCommand["betaflight-cli osd set-canvas"]
	if osdCanvas["operation"] != "write" || osdCanvas["confirmation"] != "--yes" || osdCanvas["requires_connection"] != true || osdCanvas["output_root"] != "osd_canvas" || osdCanvas["runnable"] != true {
		t.Fatalf("osd canvas capability = %+v", osdCanvas)
	}
	osdGeneralConfig := byCommand["betaflight-cli osd set-general-json"]
	if osdGeneralConfig["operation"] != "write" || osdGeneralConfig["confirmation"] != "--yes" || osdGeneralConfig["requires_connection"] != true || osdGeneralConfig["output_root"] != "osd_general_config" || osdGeneralConfig["runnable"] != true {
		t.Fatalf("osd general config capability = %+v", osdGeneralConfig)
	}
	osdVideoSystem := byCommand["betaflight-cli osd set-video-system"]
	if osdVideoSystem["operation"] != "write" || osdVideoSystem["confirmation"] != "--yes" || osdVideoSystem["requires_connection"] != true || osdVideoSystem["output_root"] != "osd_video_system" || osdVideoSystem["runnable"] != true {
		t.Fatalf("osd video system capability = %+v", osdVideoSystem)
	}
	osdVideoSystemJSON := byCommand["betaflight-cli osd set-video-system-json"]
	if osdVideoSystemJSON["operation"] != "write" || osdVideoSystemJSON["confirmation"] != "--yes" || osdVideoSystemJSON["requires_connection"] != true || osdVideoSystemJSON["output_root"] != "osd_video_system" || osdVideoSystemJSON["input"] == "" || osdVideoSystemJSON["runnable"] != true {
		t.Fatalf("osd video system json capability = %+v", osdVideoSystemJSON)
	}
	osdPosition := byCommand["betaflight-cli osd set-position"]
	if osdPosition["operation"] != "write" || osdPosition["confirmation"] != "--yes" || osdPosition["requires_connection"] != true || osdPosition["output_root"] != "osd_position" || osdPosition["runnable"] != true {
		t.Fatalf("osd position capability = %+v", osdPosition)
	}
	osdPositionJSON := byCommand["betaflight-cli osd set-position-json"]
	if osdPositionJSON["operation"] != "write" || osdPositionJSON["confirmation"] != "--yes" || osdPositionJSON["requires_connection"] != true || osdPositionJSON["output_root"] != "osd_position" || osdPositionJSON["input"] == "" || osdPositionJSON["runnable"] != true {
		t.Fatalf("osd position json capability = %+v", osdPositionJSON)
	}
	osdStat := byCommand["betaflight-cli osd set-stat"]
	if osdStat["operation"] != "write" || osdStat["confirmation"] != "--yes" || osdStat["requires_connection"] != true || osdStat["output_root"] != "osd_stat" || osdStat["runnable"] != true {
		t.Fatalf("osd stat capability = %+v", osdStat)
	}
	osdStatJSON := byCommand["betaflight-cli osd set-stat-json"]
	if osdStatJSON["operation"] != "write" || osdStatJSON["confirmation"] != "--yes" || osdStatJSON["requires_connection"] != true || osdStatJSON["output_root"] != "osd_stat" || osdStatJSON["input"] == "" || osdStatJSON["runnable"] != true {
		t.Fatalf("osd stat json capability = %+v", osdStatJSON)
	}
	osdTimer := byCommand["betaflight-cli osd set-timer"]
	if osdTimer["operation"] != "write" || osdTimer["confirmation"] != "--yes" || osdTimer["requires_connection"] != true || osdTimer["output_root"] != "osd_timer" || osdTimer["runnable"] != true {
		t.Fatalf("osd timer capability = %+v", osdTimer)
	}
	osdTimerJSON := byCommand["betaflight-cli osd set-timer-json"]
	if osdTimerJSON["operation"] != "write" || osdTimerJSON["confirmation"] != "--yes" || osdTimerJSON["requires_connection"] != true || osdTimerJSON["output_root"] != "osd_timer" || osdTimerJSON["input"] == "" || osdTimerJSON["runnable"] != true {
		t.Fatalf("osd timer json capability = %+v", osdTimerJSON)
	}
	vtxTableBand := byCommand["betaflight-cli vtxtable set-band"]
	if vtxTableBand["operation"] != "write" || vtxTableBand["confirmation"] != "--yes" || vtxTableBand["requires_connection"] != true || vtxTableBand["output_root"] != "vtxtable_band" || vtxTableBand["runnable"] != true {
		t.Fatalf("vtxtable band capability = %+v", vtxTableBand)
	}
	vtxTableBandJSON := byCommand["betaflight-cli vtxtable set-band-json"]
	if vtxTableBandJSON["operation"] != "write" || vtxTableBandJSON["confirmation"] != "--yes" || vtxTableBandJSON["requires_connection"] != true || vtxTableBandJSON["output_root"] != "vtxtable_band" || vtxTableBandJSON["input"] == "" || vtxTableBandJSON["runnable"] != true {
		t.Fatalf("vtxtable band json capability = %+v", vtxTableBandJSON)
	}
	vtxTableSet := byCommand["betaflight-cli vtxtable set-json"]
	if vtxTableSet["operation"] != "write" || vtxTableSet["confirmation"] != "--yes" || vtxTableSet["requires_connection"] != true || vtxTableSet["output_root"] != "vtxtable" || vtxTableSet["runnable"] != true {
		t.Fatalf("vtxtable set-json capability = %+v", vtxTableSet)
	}
	vtxTablePower := byCommand["betaflight-cli vtxtable set-power"]
	if vtxTablePower["operation"] != "write" || vtxTablePower["confirmation"] != "--yes" || vtxTablePower["requires_connection"] != true || vtxTablePower["output_root"] != "vtxtable_power" || vtxTablePower["runnable"] != true {
		t.Fatalf("vtxtable power capability = %+v", vtxTablePower)
	}
	vtxTablePowerJSON := byCommand["betaflight-cli vtxtable set-power-json"]
	if vtxTablePowerJSON["operation"] != "write" || vtxTablePowerJSON["confirmation"] != "--yes" || vtxTablePowerJSON["requires_connection"] != true || vtxTablePowerJSON["output_root"] != "vtxtable_power" || vtxTablePowerJSON["input"] == "" || vtxTablePowerJSON["runnable"] != true {
		t.Fatalf("vtxtable power json capability = %+v", vtxTablePowerJSON)
	}
	validate := byCommand["betaflight-cli configuration validate"]
	if validate["operation"] != "offline" || validate["requires_connection"] != false || validate["output_root"] != "configuration_validation" {
		t.Fatalf("validate capability = %+v", validate)
	}
	configuration := byCommand["betaflight-cli configuration"]
	if configuration["operation"] != "group" || configuration["runnable"] != false {
		t.Fatalf("configuration capability = %+v", configuration)
	}
	presetsFetch := byCommand["betaflight-cli presets fetch"]
	if presetsFetch["operation"] != "offline" || presetsFetch["requires_connection"] != false || presetsFetch["confirmation"] != "none" || presetsFetch["output_root"] != "change_plan" {
		t.Fatalf("presets fetch capability = %+v", presetsFetch)
	}
	workflows := capabilities["workflows"].([]any)
	foundBackup := false
	for _, item := range workflows {
		workflow := item.(map[string]any)
		if workflow["name"] == "backup-before-change" && workflow["safety_class"] == "read_only" {
			foundBackup = true
		}
	}
	if !foundBackup {
		t.Fatalf("workflows = %+v", workflows)
	}
}

func TestOfflineCommandOutputRootsMatchCapabilities(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0xaa, 0xbb}, 0o600); err != nil {
		t.Fatalf("write firmware image: %v", err)
	}

	cases := []struct {
		command string
		args    []string
		input   string
	}{
		{command: "betaflight-cli version", args: []string{"version"}},
		{command: "betaflight-cli schema", args: []string{"schema"}},
		{command: "betaflight-cli capabilities", args: []string{"capabilities"}},
		{command: "betaflight-cli capabilities coverage", args: []string{"capabilities", "coverage"}},
		{command: "betaflight-cli msp list", args: []string{"msp", "list", "--name", "MSP_NAME"}},
		{command: "betaflight-cli msp metadata", args: []string{"msp", "metadata"}},
		{command: "betaflight-cli batch plan", args: []string{"batch", "plan"}, input: "feature GPS\n"},
		{command: "betaflight-cli restore plan", args: []string{"restore", "plan"}, input: "feature GPS\n"},
		{command: "betaflight-cli presets plan", args: []string{"presets", "plan"}, input: "feature GPS\n"},
		{command: "betaflight-cli firmware flash", args: []string{"firmware", "flash", "--image", image, "--tool", "dfu-util"}},
	}

	roots := capabilityOutputRoots(t)
	for _, tt := range cases {
		t.Run(tt.command, func(t *testing.T) {
			want := roots[tt.command]
			if want == "" {
				t.Fatalf("missing capability output root for %q", tt.command)
			}
			called := false
			env, err := runTestCommandWithInput(t, tt.args, tt.input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				called = true
				return nil, connection.TargetInfo{}, nil
			})
			if err != nil {
				t.Fatalf("command error = %v", err)
			}
			if called {
				t.Fatalf("%s unexpectedly connected", tt.command)
			}
			if !env.OK {
				t.Fatalf("env.OK = false: %+v", env.Errors)
			}
			data := env.Data.(map[string]any)
			if _, ok := data[want]; !ok {
				t.Fatalf("%s data missing advertised output root %q: %+v", tt.command, want, data)
			}
		})
	}
}

func TestReadOnlyCommandOutputRootsMatchCapabilities(t *testing.T) {
	cases := []struct {
		command string
		args    []string
	}{
		{command: "betaflight-cli info", args: []string{"info"}},
		{command: "betaflight-cli status", args: []string{"status"}},
		{command: "betaflight-cli telemetry snapshot", args: []string{"telemetry", "snapshot"}},
		{command: "betaflight-cli firmware status", args: []string{"firmware", "status"}},
		{command: "betaflight-cli target status", args: []string{"target", "status"}},
		{command: "betaflight-cli text status", args: []string{"text", "status"}},
		{command: "betaflight-cli features status", args: []string{"features", "status"}},
		{command: "betaflight-cli battery status", args: []string{"battery", "status"}},
		{command: "betaflight-cli receiver status", args: []string{"receiver", "status"}},
		{command: "betaflight-cli sensors status", args: []string{"sensors", "status"}},
		{command: "betaflight-cli modes list", args: []string{"modes", "list"}},
		{command: "betaflight-cli serial status", args: []string{"serial", "status"}},
		{command: "betaflight-cli motors status", args: []string{"motors", "status"}},
		{command: "betaflight-cli profiles status", args: []string{"profiles", "status"}},
		{command: "betaflight-cli rates status", args: []string{"rates", "status"}},
		{command: "betaflight-cli filters status", args: []string{"filters", "status"}},
		{command: "betaflight-cli debug status", args: []string{"debug", "status"}},
		{command: "betaflight-cli environment status", args: []string{"environment", "status"}},
		{command: "betaflight-cli rtc status", args: []string{"rtc", "status"}},
		{command: "betaflight-cli beeper config", args: []string{"beeper", "config"}},
		{command: "betaflight-cli transponder config", args: []string{"transponder", "config"}},
		{command: "betaflight-cli vtx config", args: []string{"vtx", "config"}},
		{command: "betaflight-cli settings firmware-get", args: []string{"settings", "firmware-get", "gyro_lpf1_static_hz"}},
		{command: "betaflight-cli settings firmware-info", args: []string{"settings", "firmware-info", "gyro_lpf1_static_hz"}},
		{command: "betaflight-cli vtx device-status", args: []string{"vtx", "device-status"}},
		{command: "betaflight-cli vtxtable status", args: []string{"vtxtable", "status"}},
		{command: "betaflight-cli osd status", args: []string{"osd", "status"}},
		{command: "betaflight-cli leds status", args: []string{"leds", "status"}},
		{command: "betaflight-cli storage status", args: []string{"storage", "status"}},
		{command: "betaflight-cli system status", args: []string{"system", "status"}},
		{command: "betaflight-cli tasks status", args: []string{"tasks", "status"}},
	}

	roots := capabilityOutputRoots(t)
	for _, tt := range cases {
		t.Run(tt.command, func(t *testing.T) {
			want := roots[tt.command]
			if want == "" {
				t.Fatalf("missing capability output root for %q", tt.command)
			}
			env, err := runTestCommand(t, tt.args, nil)
			if err != nil {
				t.Fatalf("command error = %v", err)
			}
			if !env.OK {
				t.Fatalf("env.OK = false: %+v", env.Errors)
			}
			data := env.Data.(map[string]any)
			if _, ok := data[want]; !ok {
				t.Fatalf("%s data missing advertised output root %q: %+v", tt.command, want, data)
			}
		})
	}
}

func TestPlanOnlyCommandOutputRootsMatchCapabilities(t *testing.T) {
	cases := []struct {
		command string
		args    []string
		input   string
	}{
		{command: "betaflight-cli features enable", args: []string{"features", "enable", "GPS"}},
		{command: "betaflight-cli features set-json", args: []string{"features", "set-json", "-"}, input: `{"enable":["GPS"],"disable":["AIRMODE"]}`},
		{command: "betaflight-cli beeper set-json", args: []string{"beeper", "set-json", "-"}, input: `{"enable":["ARMING"],"disable":["RX_LOST"]}`},
		{command: "betaflight-cli transponder set-json", args: []string{"transponder", "set-json", "-"}, input: `{"provider":"ILAP","data":[1,2,3,4]}`},
		{command: "betaflight-cli settings set-json", args: []string{"settings", "set-json", "-"}, input: `{"gyro_lpf1_static_hz":0}`},
	}

	roots := capabilityOutputRoots(t)
	for _, tt := range cases {
		t.Run(tt.command, func(t *testing.T) {
			want := roots[tt.command]
			if want == "" {
				t.Fatalf("missing capability output root for %q", tt.command)
			}
			called := false
			env, err := runTestCommandWithInput(t, tt.args, tt.input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				called = true
				return nil, connection.TargetInfo{}, nil
			})
			if err != nil {
				t.Fatalf("command error = %v", err)
			}
			if called {
				t.Fatalf("%s unexpectedly connected in plan-only mode", tt.command)
			}
			if !env.OK {
				t.Fatalf("env.OK = false: %+v", env.Errors)
			}
			data := env.Data.(map[string]any)
			if _, ok := data[want]; !ok {
				t.Fatalf("%s data missing advertised output root %q: %+v", tt.command, want, data)
			}
		})
	}
}

func TestConfirmedWriteCommandOutputRootsMatchCapabilities(t *testing.T) {
	cases := []struct {
		command string
		args    []string
		input   string
	}{
		{command: "betaflight-cli text set", args: []string{"text", "set", "craft_name", "Quad", "--yes"}},
		{command: "betaflight-cli rtc set", args: []string{"rtc", "set", "--timestamp", "2026-06-23T12:34:56.789Z", "--yes"}},
		{command: "betaflight-cli debug set-accelerometer-trim", args: []string{"--yes", "debug", "set-accelerometer-trim", "--", "-12", "34"}},
		{command: "betaflight-cli features set-mask", args: []string{"features", "set-mask", "0x00040488", "--yes"}},
		{command: "betaflight-cli vtx set-config", args: []string{"vtx", "set-config", "5", "8", "2", "true", "5861", "2", "5662", "--yes"}},
		{command: "betaflight-cli receiver set-map", args: []string{"receiver", "set-map", "0", "1", "3", "2", "--yes"}},
	}

	roots := capabilityOutputRoots(t)
	for _, tt := range cases {
		t.Run(tt.command, func(t *testing.T) {
			want := roots[tt.command]
			if want == "" {
				t.Fatalf("missing capability output root for %q", tt.command)
			}
			env, err := runTestCommandWithInput(t, tt.args, tt.input, nil)
			if err != nil {
				t.Fatalf("command error = %v", err)
			}
			if !env.OK {
				t.Fatalf("env.OK = false: %+v", env.Errors)
			}
			data := env.Data.(map[string]any)
			if _, ok := data[want]; !ok {
				t.Fatalf("%s data missing advertised output root %q: %+v", tt.command, want, data)
			}
		})
	}
}

func TestDangerousCommandOutputRootsMatchCapabilities(t *testing.T) {
	cases := []struct {
		command        string
		args           []string
		wantSideEffect string
	}{
		{command: "betaflight-cli motors test-plan", args: []string{"motors", "test-plan", "--motor", "1", "--value", "1000", "--duration", "1s"}},
		{command: "betaflight-cli storage erase", args: []string{"storage", "erase", "--yes"}, wantSideEffect: "dataflash_erase"},
		{command: "betaflight-cli save", args: []string{"save", "--yes"}, wantSideEffect: "save"},
		{command: "betaflight-cli sensors calibrate-accelerometer", args: []string{"sensors", "calibrate-accelerometer", "--yes"}, wantSideEffect: "sensor_calibration"},
		{command: "betaflight-cli sensors calibrate-magnetometer", args: []string{"sensors", "calibrate-magnetometer", "--yes"}, wantSideEffect: "sensor_calibration"},
		{command: "betaflight-cli reboot firmware", args: []string{"reboot", "firmware", "--yes"}, wantSideEffect: "firmware_reboot"},
		{command: "betaflight-cli reboot bootloader", args: []string{"reboot", "bootloader", "--yes"}, wantSideEffect: "bootloader_reboot"},
		{command: "betaflight-cli reboot bootloader-flash", args: []string{"reboot", "bootloader-flash", "--yes"}, wantSideEffect: "bootloader_reboot"},
		{command: "betaflight-cli reboot msc", args: []string{"reboot", "msc", "--yes"}, wantSideEffect: "msc_reboot"},
		{command: "betaflight-cli reboot msc-utc", args: []string{"reboot", "msc-utc", "--yes"}, wantSideEffect: "msc_reboot"},
	}

	roots := capabilityOutputRoots(t)
	for _, tt := range cases {
		t.Run(tt.command, func(t *testing.T) {
			want := roots[tt.command]
			if want == "" {
				t.Fatalf("missing capability output root for %q", tt.command)
			}
			env, err := runTestCommand(t, tt.args, nil)
			if err != nil {
				t.Fatalf("command error = %v", err)
			}
			if !env.OK {
				t.Fatalf("env.OK = false: %+v", env.Errors)
			}
			data := env.Data.(map[string]any)
			if _, ok := data[want]; !ok {
				t.Fatalf("%s data missing advertised output root %q: %+v", tt.command, want, data)
			}
			if tt.wantSideEffect == "" {
				if len(env.SideEffects) != 0 {
					t.Fatalf("%s side effects = %+v, want none", tt.command, env.SideEffects)
				}
				return
			}
			if len(env.SideEffects) != 1 || env.SideEffects[0].Type != tt.wantSideEffect {
				t.Fatalf("%s side effects = %+v, want type %q", tt.command, env.SideEffects, tt.wantSideEffect)
			}
		})
	}
}

func TestCapabilitiesCoverageReportsParityDomains(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"capabilities", "coverage"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if called {
		t.Fatal("connector was called for capabilities coverage")
	}
	data := env.Data.(map[string]any)
	coverage := data["coverage"].(map[string]any)
	summary := coverage["summary"].(map[string]any)
	if summary["domain_count"].(float64) < 15 || summary["implemented_count"].(float64) < 10 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary["blackbox_domains"].(float64) < 1 {
		t.Fatalf("summary blackbox_domains = %v, want at least 1", summary["blackbox_domains"])
	}
	domains := coverage["domains"].([]any)
	if len(domains) != int(summary["domain_count"].(float64)) {
		t.Fatalf("coverage domain_count mismatch: got=%d want=%v", len(domains), summary["domain_count"])
	}
	domainCounts := map[string]int{}
	byDomain := map[string]map[string]any{}
	for _, item := range domains {
		domain := item.(map[string]any)
		name := domain["domain"].(string)
		domainCounts[name]++
		byDomain[domain["domain"].(string)] = domain
	}
	for name, count := range domainCounts {
		if count != 1 {
			t.Fatalf("coverage domain %q has %d entries, expected 1", name, count)
		}
	}
	backup := byDomain["configuration-backup"]
	if backup["status"] != "implemented" || len(backup["read_commands"].([]any)) == 0 {
		t.Fatalf("backup domain = %+v", backup)
	}
	maintenance := byDomain["firmware-maintenance"]
	if maintenance["status"] != "implemented" || len(maintenance["dangerous_commands"].([]any)) == 0 {
		t.Fatalf("maintenance domain = %+v", maintenance)
	}
	flashing := byDomain["firmware-flashing"]
	if flashing["status"] != "implemented" || len(flashing["dangerous_commands"].([]any)) == 0 || !containsAnyString(flashing["output_roots"].([]any), "firmware_flash") {
		t.Fatalf("firmware-flashing domain = %+v", flashing)
	}
	motors := byDomain["motors-servos-mixer"]
	if motors["status"] != "implemented" {
		t.Fatalf("motors domain = %+v", motors)
	}
	settings := byDomain["settings"]
	if settings["status"] != "implemented" || len(settings["read_commands"].([]any)) == 0 || len(settings["write_commands"].([]any)) == 0 {
		t.Fatalf("settings domain = %+v", settings)
	}
	beeperTransponder := byDomain["beeper-transponder"]
	if beeperTransponder["status"] != "implemented" || !containsAnyString(beeperTransponder["output_roots"].([]any), "change_plan") {
		t.Fatalf("beeper-transponder domain = %+v", beeperTransponder)
	}
	storageBlackbox := byDomain["storage-blackbox"]
	if storageBlackbox["status"] != "implemented" || !containsAnyString(storageBlackbox["output_roots"].([]any), "blackbox") {
		t.Fatalf("storage-blackbox domain = %+v", storageBlackbox)
	}
	gaps := coverage["next_gaps"].([]any)
	for _, item := range gaps {
		gap := item.(map[string]any)
		if gap["domain"] == "blackbox-decoding" {
			t.Fatalf("unexpected blackbox-decoding gap remains: %+v", gap)
		}
		if gap["domain"] == "motor-testing" {
			t.Fatalf("unexpected motor-testing gap remains: %+v", gap)
		}
	}
	if _, ok := byDomain["firmware-flashing"]; !ok {
		t.Fatalf("missing firmware-flashing domain in coverage")
	}
}

func TestCapabilitiesCoverageIncludesRunnableEnvelopeCommands(t *testing.T) {
	schemaEnv, err := runTestCommand(t, []string{"schema"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("schema unexpectedly connected")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("schema command error = %v", err)
	}
	coverageEnv, err := runTestCommand(t, []string{"capabilities", "coverage"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("coverage unexpectedly connected")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("coverage command error = %v", err)
	}

	covered := coverageCommandSet(t, coverageEnv)
	for command, row := range schemaCommandContractRows(t, schemaEnv) {
		if row["runnable"] != true {
			continue
		}
		operation, _ := row["operation"].(string)
		if operation == "text_output" {
			continue
		}
		if _, ok := covered[command]; !ok {
			t.Fatalf("runnable envelope command %q is missing from capabilities coverage", command)
		}
	}
}

func TestRegistryCapabilityPathsMatchCommandTree(t *testing.T) {
	root := (&app{}).rootCommand()
	commandPaths := make(map[string]struct{})
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Hidden {
			return
		}
		commandPaths[commandPath(cmd)] = struct{}{}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)

	for path := range capabilityMetadataRegistry() {
		if path == "betaflight-cli capabilities" {
			continue
		}
		if _, ok := commandPaths[path]; !ok {
			t.Fatalf("capability registry path %q is not present in the command tree", path)
		}
	}
}

func containsAnyString(values []any, want string) bool {
	for _, value := range values {
		if text, ok := value.(string); ok && text == want {
			return true
		}
	}
	return false
}

func capabilityOutputRoots(t *testing.T) map[string]string {
	t.Helper()
	env, err := runTestCommand(t, []string{"capabilities"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("capabilities unexpectedly connected")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("capabilities command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("capabilities env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	capabilities := data["capabilities"].(map[string]any)
	commands := capabilities["commands"].([]any)
	roots := map[string]string{}
	for _, item := range commands {
		command := item.(map[string]any)
		root, _ := command["output_root"].(string)
		roots[command["command"].(string)] = root
	}
	return roots
}

func schemaCommandContractRows(t *testing.T, env output.Envelope) map[string]map[string]any {
	t.Helper()
	data := env.Data.(map[string]any)
	schema := data["schema"].(map[string]any)
	contracts := schema["command_contracts"].(map[string]any)
	return commandRowsByCommand(t, contracts["commands"].([]any))
}

func capabilityCommandRows(t *testing.T, env output.Envelope) map[string]map[string]any {
	t.Helper()
	data := env.Data.(map[string]any)
	capabilities := data["capabilities"].(map[string]any)
	return commandRowsByCommand(t, capabilities["commands"].([]any))
}

func coverageCommandSet(t *testing.T, env output.Envelope) map[string]struct{} {
	t.Helper()
	data := env.Data.(map[string]any)
	coverage := data["coverage"].(map[string]any)
	domains := coverage["domains"].([]any)
	out := map[string]struct{}{}
	for _, item := range domains {
		domain := item.(map[string]any)
		for _, field := range []string{"read_commands", "write_commands", "dangerous_commands"} {
			rows, _ := domain[field].([]any)
			for _, row := range rows {
				command, ok := row.(string)
				if !ok || command == "" {
					t.Fatalf("coverage command row %s = %+v", field, row)
				}
				out[command] = struct{}{}
			}
		}
	}
	return out
}

func commandRowsByCommand(t *testing.T, rows []any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, item := range rows {
		row := item.(map[string]any)
		command, ok := row["command"].(string)
		if !ok || command == "" {
			t.Fatalf("command row missing command: %+v", row)
		}
		if _, exists := out[command]; exists {
			t.Fatalf("duplicate command row for %q", command)
		}
		out[command] = row
	}
	return out
}

func operationNeedsConfirmation(operation string) bool {
	switch operation {
	case "write", "write_when_apply_is_set", "plan_or_write", "plan_or_dangerous_execute", "dangerous", "read_only_or_write_or_dangerous":
		return true
	default:
		return false
	}
}
