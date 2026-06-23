package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FirmwareFlashPlan struct {
	ImagePath        string   `json:"image_path"`
	ImageSizeBytes   int64    `json:"image_size_bytes"`
	ImageSHA256      string   `json:"image_sha256"`
	Tool             string   `json:"tool"`
	ToolArgs         []string `json:"tool_args"`
	RebootToBootloader bool  `json:"reboot_to_bootloader"`
	RebootCommand    string   `json:"reboot_command"`
	EstimatedCommand []string `json:"estimated_command"`
}

type FirmwareFlashResult struct {
	Plan        FirmwareFlashPlan `json:"plan"`
	Executed    bool             `json:"executed"`
	Rebooted    bool             `json:"rebooted"`
	Output      string           `json:"output"`
	ExitCode    int              `json:"exit_code"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt time.Time        `json:"completed_at"`
}

type FirmwareFlashOptions struct {
	ImagePath          string
	Tool               string
	ToolArgs           []string
	RebootToBootloader bool
}

func PlanFirmwareFlash(opts FirmwareFlashOptions) (*FirmwareFlashPlan, error) {
	if strings.TrimSpace(opts.ImagePath) == "" {
		return nil, fmt.Errorf("missing --image")
	}
	imagePath := filepath.Clean(opts.ImagePath)
	info, err := os.Stat(imagePath)
	if err != nil {
		return nil, fmt.Errorf("image file unavailable: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("image path %q is a directory", imagePath)
	}
	if strings.TrimSpace(opts.Tool) == "" {
		opts.Tool = "dfu-util"
	}
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("open image %q: %w", imagePath, err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, fmt.Errorf("hash firmware image: %w", err)
	}
	command := []string{
		opts.Tool,
	}
	command = append(command, opts.ToolArgs...)
	command = append(command, imagePath)
	rebootCmd := "none"
	if opts.RebootToBootloader {
		rebootCmd = "bootloader_flash"
	}
	return &FirmwareFlashPlan{
		ImagePath:          imagePath,
		ImageSizeBytes:     info.Size(),
		ImageSHA256:        hex.EncodeToString(hasher.Sum(nil)),
		Tool:               opts.Tool,
		ToolArgs:           append([]string(nil), opts.ToolArgs...),
		RebootToBootloader: opts.RebootToBootloader,
		RebootCommand:      rebootCmd,
		EstimatedCommand:    command,
	}, nil
}

func ExecuteFirmwareFlash(ctx context.Context, plan FirmwareFlashPlan) (FirmwareFlashResult, error) {
	toolPath, err := exec.LookPath(plan.Tool)
	if err != nil {
		return FirmwareFlashResult{}, fmt.Errorf("flash tool %q not found in PATH", plan.Tool)
	}
	args := append([]string{}, plan.ToolArgs...)
	args = append(args, plan.ImagePath)
	cmd := exec.CommandContext(ctx, toolPath, args...)
	started := time.Now()
	output, err := cmd.CombinedOutput()
	result := FirmwareFlashResult{
		Plan:        plan,
		Executed:    true,
		StartedAt:   started,
		Output:      strings.TrimSpace(string(output)),
	}
	result.CompletedAt = time.Now()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		return result, fmt.Errorf("firmware flash command failed: %w", err)
	}
	return result, nil
}
