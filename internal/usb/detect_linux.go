//go:build linux

package usb

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// DetectUSBDrives finds removable USB drives on Linux by reading /proc/mounts.
func DetectUSBDrives() ([]USBDrive, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("open /proc/mounts: %w", err)
	}
	defer f.Close()

	var drives []USBDrive
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		if !strings.HasPrefix(device, "/dev/sd") {
			continue
		}

		baseDev := strings.TrimRight(strings.TrimPrefix(device, "/dev/"), "0123456789")
		removablePath := fmt.Sprintf("/sys/block/%s/removable", baseDev)

		data, err := os.ReadFile(removablePath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) != "1" {
			continue
		}

		totalBytes, freeBytes := dfSpace(mountPoint)

		drives = append(drives, USBDrive{
			Path:       mountPoint,
			Label:      baseDev,
			SizeBytes:  totalBytes,
			FreeBytes:  freeBytes,
			FileSystem: fsType,
		})
	}

	return drives, scanner.Err()
}

func dfSpace(mountPoint string) (int64, int64) {
	cmd := exec.Command("df", "-k", mountPoint)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return 0, 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, 0
	}
	total, _ := strconv.ParseInt(fields[1], 10, 64)
	avail, _ := strconv.ParseInt(fields[3], 10, 64)
	return total * 1024, avail * 1024
}
