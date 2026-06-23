package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanFirmwareFlashRequiresImage(t *testing.T) {
	_, err := PlanFirmwareFlash(FirmwareFlashOptions{})
	if err == nil || err.Error() == "" {
		t.Fatalf("expected validation error, got nil")
	}
}

func TestPlanFirmwareFlashRejectsMissingImage(t *testing.T) {
	_, err := PlanFirmwareFlash(FirmwareFlashOptions{
		ImagePath: filepath.Join(t.TempDir(), "missing.bin"),
	})
	if err == nil {
		t.Fatalf("expected error for missing image")
	}
}

func TestPlanFirmwareFlashWithTempImage(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0x12, 0x34, 0x56}, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	plan, err := PlanFirmwareFlash(FirmwareFlashOptions{
		ImagePath: image,
		Tool:      "dfu-util",
	})
	if err != nil {
		t.Fatalf("PlanFirmwareFlash() error = %v", err)
	}
	if plan.ImagePath != image || plan.Tool != "dfu-util" || plan.ImageSizeBytes != 3 {
		t.Fatalf("unexpected plan = %+v", plan)
	}
	if len(plan.ToolArgs) != 0 || len(plan.EstimatedCommand) != 2 {
		t.Fatalf("unexpected estimated command = %+v", plan.EstimatedCommand)
	}
}

func TestExecuteFirmwareFlashNeedsToolOnPath(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0x00}, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	plan, err := PlanFirmwareFlash(FirmwareFlashOptions{
		ImagePath: image,
		Tool:      "definitely-not-a-real-tool",
	})
	if err != nil {
		t.Fatalf("PlanFirmwareFlash() error = %v", err)
	}
	if _, err := ExecuteFirmwareFlash(context.Background(), *plan); err == nil {
		t.Fatalf("expected tool-not-found error")
	}
}
