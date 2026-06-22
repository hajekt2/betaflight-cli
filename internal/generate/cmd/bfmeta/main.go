package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hajekt2/betaflight-cli/internal/generate"
)

func main() {
	var sourceRoot string
	var sourceVersion string
	var mspOut string
	var settingsOut string
	flag.StringVar(&sourceRoot, "betaflight-src", "", "path to a Betaflight source checkout")
	flag.StringVar(&sourceVersion, "source-version", "", "Betaflight tag or commit used for generated metadata")
	flag.StringVar(&mspOut, "out-msp", "pkg/msp/codes_generated.go", "generated MSP registry output path")
	flag.StringVar(&settingsOut, "out-settings", "internal/settings/metadata_generated.go", "generated settings metadata output path")
	flag.Parse()

	if sourceRoot == "" {
		sourceRoot = os.Getenv("BETAFLIGHT_SRC")
	}
	if sourceVersion == "" {
		sourceVersion = os.Getenv("BETAFLIGHT_VERSION")
	}
	if sourceVersion == "" {
		sourceVersion = "unknown"
	}
	if sourceRoot == "" {
		fatalf("missing -betaflight-src or BETAFLIGHT_SRC")
	}

	if err := run(sourceRoot, sourceVersion, mspOut, settingsOut); err != nil {
		fatalf("%v", err)
	}
}

func run(sourceRoot, sourceVersion, mspOut, settingsOut string) error {
	mspFiles := []string{
		"src/main/msp/msp_protocol.h",
		"src/main/msp/msp_protocol_v2_common.h",
		"src/main/msp/msp_protocol_v2_betaflight.h",
	}
	var commands []generate.MSPCommand
	for _, sourceFile := range mspFiles {
		file, err := os.Open(filepath.Join(sourceRoot, sourceFile))
		if err != nil {
			return err
		}
		parsed, err := generate.ParseMSPCommands(sourceFile, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		commands = append(commands, parsed...)
	}
	mspBytes, err := generate.WriteMSPRegistry(commands, sourceVersion)
	if err != nil {
		return err
	}
	if err := writeFile(mspOut, mspBytes); err != nil {
		return err
	}

	settingsCPath := "src/main/cli/settings.c"
	settingsHPath := "src/main/cli/settings.h"
	parameterNamesPath := "src/main/fc/parameter_names.h"
	settingsC, err := os.ReadFile(filepath.Join(sourceRoot, settingsCPath))
	if err != nil {
		return err
	}
	settingsH, err := os.ReadFile(filepath.Join(sourceRoot, settingsHPath))
	if err != nil {
		return err
	}
	parameterNames, err := os.ReadFile(filepath.Join(sourceRoot, parameterNamesPath))
	if err != nil {
		return err
	}
	parsedSettings, err := generate.ParseSettings(generate.SettingsParseInput{
		SettingsC:      string(settingsC),
		SettingsH:      string(settingsH),
		ParameterNames: string(parameterNames),
		SourceC:        settingsCPath,
		SourceH:        settingsHPath,
		SourceParams:   parameterNamesPath,
	})
	if err != nil {
		return err
	}
	for _, warning := range parsedSettings.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	settingsBytes, err := generate.WriteSettingsRegistry(parsedSettings.Settings, sourceVersion, []string{settingsCPath, settingsHPath, parameterNamesPath})
	if err != nil {
		return err
	}
	return writeFile(settingsOut, settingsBytes)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "bfmeta: "+format+"\n", args...)
	os.Exit(1)
}
