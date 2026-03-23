package usb

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestDetectUSBDrives(t *testing.T) {
	// First show raw diskutil output for debugging
	cmd := exec.Command("diskutil", "list", "external")
	rawOut, err := cmd.Output()
	if err != nil {
		t.Logf("diskutil list external error: %v", err)
	} else {
		t.Logf("Raw diskutil output:\n%s", string(rawOut))
	}

	// Debug: manually parse to see what we find
	output := strings.TrimSpace(string(rawOut))
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			t.Logf("  Skip (too few fields): %q", line)
			continue
		}
		lastField := fields[len(fields)-1]
		t.Logf("  Line lastField=%q hasPrefix(disk)=%v contains(s)=%v",
			lastField,
			strings.HasPrefix(lastField, "disk"),
			strings.Contains(lastField, "s"))

		if strings.HasPrefix(lastField, "disk") && strings.Contains(lastField, "s") {
			idx := strings.Index(lastField, "s")
			suffix := lastField[idx+1:]
			_, atoiErr := strconv.Atoi(suffix)
			t.Logf("    idx=%d suffix=%q atoiErr=%v", idx, suffix, atoiErr)
		}
	}

	drives, err := DetectUSBDrives()
	if err != nil {
		t.Logf("Detection error: %v", err)
		return
	}
	t.Logf("Found %d drive(s):", len(drives))
	for i, d := range drives {
		t.Logf("  [%d] Path=%s Label=%s Size=%d Free=%d FS=%s",
			i+1, d.Path, d.Label, d.SizeBytes, d.FreeBytes, d.FileSystem)
	}
}
